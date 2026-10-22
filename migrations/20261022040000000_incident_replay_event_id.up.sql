-- Which event a replay actually submitted.
--
-- incidents.event_id points at the most recent occurrence and moves on every
-- merge, so a replay's outcome could not be judged by it: a different event
-- failing under the same fingerprint both redirected the success check at the
-- wrong event and — through last_seen_at — was reported as the replay having
-- failed. replay_event_id is stamped when the replay is requested and never
-- moves, so both signals are asked of the event that was replayed.
ALTER TABLE incidents ADD COLUMN replay_event_id uuid NULL;

-- Replays already in flight: event_id is still the best available answer for
-- them, and leaving it NULL would strand them as permanently pending.
UPDATE incidents SET replay_event_id = event_id WHERE replay_status = 'pending';
