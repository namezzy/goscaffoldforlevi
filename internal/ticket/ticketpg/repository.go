// Package ticketpg is the PostgreSQL implementation of the ticket
// repository.
package ticketpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/namezzy/goscaffoldforlevi/internal/apperr"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres"
	"github.com/namezzy/goscaffoldforlevi/internal/postgres/db"
	"github.com/namezzy/goscaffoldforlevi/internal/ticket"
)

const codeForeignKeyViolation = "23503"

// Repository stores tickets.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a Repository backed by pool.
func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) q(ctx context.Context) *db.Queries {
	return db.New(postgres.Querier(ctx, r.pool))
}

// Create inserts a ticket.
func (r *Repository) Create(ctx context.Context, t ticket.Ticket) (ticket.Ticket, error) {
	id, err := uuid.Parse(t.ID)
	if err != nil {
		return ticket.Ticket{}, apperr.Internal(fmt.Errorf("ticket id: %w", err))
	}
	requesterID, err := uuid.Parse(t.RequesterID)
	if err != nil {
		return ticket.Ticket{}, apperr.Internal(fmt.Errorf("requester id: %w", err))
	}
	row, err := r.q(ctx).CreateTicket(ctx, db.CreateTicketParams{
		ID: id, Title: t.Title, Description: t.Description, Category: t.Category,
		Priority: string(t.Priority), RequesterID: requesterID,
	})
	if err != nil {
		return ticket.Ticket{}, translate(err, "create ticket")
	}
	return toDomain(row), nil
}

// GetByID loads a ticket by primary key.
func (r *Repository) GetByID(ctx context.Context, id string) (ticket.Ticket, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ticket.Ticket{}, errTicketNotFound()
	}
	row, err := r.q(ctx).GetTicketByID(ctx, parsed)
	if err != nil {
		return ticket.Ticket{}, translate(err, "get ticket by id")
	}
	return toDomain(row), nil
}

// ListFiltered returns one page of tickets matching f, most recent first
// unless f overrides the sort.
func (r *Repository) ListFiltered(ctx context.Context, f ticket.ListFilter, limit, offset int32) ([]ticket.Ticket, error) {
	requesterID, err := parseOptionalUUID(f.RequesterID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("requester id filter: %w", err))
	}
	assigneeID, err := parseOptionalUUID(f.AssigneeID)
	if err != nil {
		return nil, apperr.Internal(fmt.Errorf("assignee id filter: %w", err))
	}
	rows, err := r.q(ctx).ListTicketsFiltered(ctx, db.ListTicketsFilteredParams{
		Limit: limit, Offset: offset, RequesterID: requesterID, AssigneeID: assigneeID,
		Status: statusFilter(f.Status), Priority: priorityFilter(f.Priority),
		Search: nullableString(f.Search), SortBy: string(f.SortBy), SortDesc: f.SortDesc,
	})
	if err != nil {
		return nil, translate(err, "list filtered tickets")
	}
	out := make([]ticket.Ticket, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}

// CountFiltered returns the total number of tickets matching f, for
// pagination metadata.
func (r *Repository) CountFiltered(ctx context.Context, f ticket.ListFilter) (int64, error) {
	requesterID, err := parseOptionalUUID(f.RequesterID)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("requester id filter: %w", err))
	}
	assigneeID, err := parseOptionalUUID(f.AssigneeID)
	if err != nil {
		return 0, apperr.Internal(fmt.Errorf("assignee id filter: %w", err))
	}
	n, err := r.q(ctx).CountTicketsFiltered(ctx, db.CountTicketsFilteredParams{
		RequesterID: requesterID, AssigneeID: assigneeID,
		Status: statusFilter(f.Status), Priority: priorityFilter(f.Priority), Search: nullableString(f.Search),
	})
	if err != nil {
		return 0, translate(err, "count filtered tickets")
	}
	return n, nil
}

// CountByStatus reports how many tickets are in each status currently
// present, for the workbench's admin overview.
func (r *Repository) CountByStatus(ctx context.Context) (map[ticket.Status]int64, error) {
	rows, err := r.q(ctx).CountTicketsByStatus(ctx)
	if err != nil {
		return nil, translate(err, "count tickets by status")
	}
	out := make(map[ticket.Status]int64, len(rows))
	for _, row := range rows {
		out[ticket.Status(row.Status)] = row.Total
	}
	return out, nil
}

// Update applies an optimistic-concurrency-conditioned patch. A row count
// of zero from the underlying query — surfaced by pgx as ErrNoRows since
// UpdateTicket is :one — means expectedVersion no longer matches the
// stored row: someone else's action landed first. That is reported as
// TICKET_VERSION_CONFLICT, distinct from "ticket not found," because by
// the time Update is called the service has already loaded the ticket
// successfully and tickets are never deleted.
func (r *Repository) Update(ctx context.Context, id string, expectedVersion int32, p ticket.Patch) (ticket.Ticket, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return ticket.Ticket{}, errTicketNotFound()
	}
	assigneeID, err := parseOptionalUUID(p.AssigneeID)
	if err != nil {
		return ticket.Ticket{}, apperr.Internal(fmt.Errorf("assignee id: %w", err))
	}
	params := db.UpdateTicketParams{
		ClearAssignee: p.ClearAssignee, AssigneeID: assigneeID, ID: parsedID, ExpectedVersion: expectedVersion,
	}
	if p.Status != nil {
		s := string(*p.Status)
		params.Status = &s
	}
	row, err := r.q(ctx).UpdateTicket(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ticket.Ticket{}, errVersionConflict()
		}
		return ticket.Ticket{}, translate(err, "update ticket")
	}
	return toDomain(row), nil
}

func toDomain(row db.Ticket) ticket.Ticket {
	t := ticket.Ticket{
		ID: row.ID.String(), Title: row.Title, Description: row.Description, Category: row.Category,
		Priority: ticket.Priority(row.Priority), Status: ticket.Status(row.Status),
		RequesterID: row.RequesterID.String(), Version: row.Version,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ClosedAt: row.ClosedAt,
	}
	if row.AssigneeID != nil {
		s := row.AssigneeID.String()
		t.AssigneeID = &s
	}
	return t
}

func parseOptionalUUID(s *string) (*uuid.UUID, error) {
	if s == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func statusFilter(s *ticket.Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func priorityFilter(p *ticket.Priority) *string {
	if p == nil {
		return nil
	}
	v := string(*p)
	return &v
}

func errTicketNotFound() error {
	return apperr.NotFound("TICKET_NOT_FOUND", "ticket not found")
}

func errVersionConflict() error {
	return apperr.Conflict("TICKET_VERSION_CONFLICT",
		"this ticket was changed by someone else; reload it and try again")
}

// translate converts a driver error into an application error, matching
// userpg's translate — see its doc comment for the reasoning.
func translate(err error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errTicketNotFound()
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == codeForeignKeyViolation {
		return apperr.Wrap(err, apperr.KindConflict, "REFERENCE_CONFLICT",
			"the request references a resource that does not exist")
	}
	return apperr.Internal(fmt.Errorf("%s: %w", op, err))
}
