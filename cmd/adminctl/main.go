// Command adminctl creates the first administrator.
//
// Until this existed, standing up the service meant generating an Argon2id
// hash by hand and writing a row with raw SQL. That is error-prone in a way
// that matters: a mistyped hash produces an account nobody can log in to, and
// a hash pasted from elsewhere produces one whose password is known elsewhere.
//
// The command is not a general user-management tool. It refuses to run once an
// administrator exists, because a command that mints privileged accounts
// outside the API is a standing bypass of both authorization and the audit
// trail. After the first administrator, use POST /api/v1/users.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/go-enterprise/internal/auth"
	"github.com/example/go-enterprise/internal/config"
	"github.com/example/go-enterprise/internal/messaging"
	"github.com/example/go-enterprise/internal/postgres"
	"github.com/example/go-enterprise/internal/user"
	"github.com/example/go-enterprise/internal/user/userpg"
)

func main() { os.Exit(run()) }

func run() int {
	var (
		email         = flag.String("email", "", "email address of the administrator to create")
		name          = flag.String("name", "Administrator", "display name")
		passwordStdin = flag.Bool("password-stdin", false,
			"read the password from stdin instead of generating one")
	)
	flag.Parse()

	if *email == "" {
		fmt.Fprintln(os.Stderr, "usage: adminctl -email <address> [-name <name>] [-password-stdin]")
		return 2
	}

	if err := createAdmin(*email, *name, *passwordStdin); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func createAdmin(email, name string, passwordStdin bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// The password is generated rather than prompted for by default. One typed
	// on a command line ends up in the shell history and in the process list;
	// one generated here is shown once and stored nowhere.
	password, err := resolvePassword(passwordStdin)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect to the database: %w", err)
	}
	defer pool.Close()

	params := auth.Argon2Params{
		Time:    cfg.Auth.Argon2Time,
		Memory:  cfg.Auth.Argon2Memory,
		Threads: cfg.Auth.Argon2Threads,
		SaltLen: auth.DefaultArgon2Params.SaltLen,
		KeyLen:  auth.DefaultArgon2Params.KeyLen,
	}

	created, err := bootstrap(ctx, pool, params, user.CreateInput{
		Email:    email,
		Name:     name,
		Password: password,
		Role:     auth.RoleAdmin,
	})
	if err != nil {
		return err
	}

	fmt.Printf("created administrator %s (%s)\n", created.Email, created.ID)
	if !passwordStdin {
		fmt.Printf("password: %s\n", password)
		fmt.Println("This is the only time it is shown. Store it now.")
	}
	return nil
}

// ErrAdminExists is returned when the bootstrap has already been done. It is a
// named error so the test can assert on the refusal rather than on wording.
var ErrAdminExists = errors.New(
	"an administrator already exists; create further accounts through POST /api/v1/users")

// bootstrap creates the administrator. It takes a pool rather than reading the
// configuration itself so that the part worth testing — the refusal, and the
// fact that the created account is a normally-hashed, loginable one — can be
// tested against a real database without a full environment.
func bootstrap(
	ctx context.Context,
	pool *pgxpool.Pool,
	params auth.Argon2Params,
	in user.CreateInput,
) (user.User, error) {
	repo := userpg.New(pool)

	admins, err := repo.CountByRole(ctx, auth.RoleAdmin)
	if err != nil {
		return user.User{}, fmt.Errorf("count administrators: %w", err)
	}
	if admins > 0 {
		return user.User{}, ErrAdminExists
	}

	hasher, err := auth.NewArgon2Hasher(params)
	if err != nil {
		return user.User{}, fmt.Errorf("build the password hasher: %w", err)
	}

	// The account is created through the service rather than with an INSERT, so
	// that it gets the same validation, email normalisation and hashing as
	// every other account. A bootstrap path that writes its own row is a
	// bootstrap path that drifts from the real one.
	// The event is discarded rather than written to the outbox: bootstrapping
	// runs before any worker exists, so an outbox row here would sit
	// undelivered until someone happened to deploy one. The administrator is
	// also not a user anything downstream needs to be told about — nobody is
	// waiting to send them a welcome email.
	svc := user.NewService(
		repo,
		hasher,
		postgres.NewTxManager(pool),
		messaging.NewNoopPublisher(slog.New(slog.NewTextHandler(io.Discard, nil))),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	return svc.Create(bootstrapContext(ctx), in)
}

// bootstrapContext supplies the principal the service's authorization check
// requires.
//
// This is not a privilege escalation dressed up as plumbing: running the
// command already requires the database credentials, which is strictly more
// power than user:create. Going through the normal authorization path — rather
// than adding a "skip the check" option to the service — keeps the exception
// visible here instead of inside the service, where it would be reachable by
// every caller.
func bootstrapContext(ctx context.Context) context.Context {
	return auth.ContextWithPrincipal(ctx, auth.Principal{
		UserID: "bootstrap",
		Role:   auth.RoleAdmin,
	})
}

func resolvePassword(fromStdin bool) (string, error) {
	if !fromStdin {
		return generatePassword()
	}

	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<10))
	if err != nil {
		return "", fmt.Errorf("read the password from stdin: %w", err)
	}
	password := strings.TrimRight(string(raw), "\r\n")
	if password == "" {
		return "", errors.New("no password was supplied on stdin")
	}
	return password, nil
}

// generatePassword returns 32 random characters. The service enforces the
// length policy; this is well above it, and entropy is what matters for an
// account whose password nobody will ever type twice.
func generatePassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate a password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
