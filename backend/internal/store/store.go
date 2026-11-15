// Package store holds all SQL for families, members, invites and devices.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"littlevillage/internal/auth"
)

// MaxMembers is the most members one family can have.
const MaxMembers = 10

var (
	// ErrNotFound means the session, invite or record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrFamilyFull means the family already has MaxMembers members.
	ErrFamilyFull = errors.New("family is full")
	// ErrNotDeviceMember means the member was not created on this device.
	ErrNotDeviceMember = errors.New("member does not belong to this device")
)

// Store runs queries against PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Ping checks that the database answers.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Member is a person in a family.
type Member struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	IsOwner      bool   `json:"is_owner"`
}

// FamilyRef identifies a family.
type FamilyRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DeviceMember is a member this browser can switch to.
type DeviceMember struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	FamilyName   string `json:"family_name"`
}

// Me describes the active member of a device.
type Me struct {
	Member        Member         `json:"member"`
	Family        FamilyRef      `json:"family"`
	DeviceMembers []DeviceMember `json:"device_members"`
}

// Family is a family with all its members and its invite token.
type Family struct {
	Family      FamilyRef
	Members     []Member
	InviteToken string
}

// Invite is what a person sees before joining by link.
type Invite struct {
	FamilyName string `json:"family_name"`
	InvitedBy  string `json:"invited_by"`
}

// Binding is the result of creating or joining a family: the new session,
// plus a fresh device token when the request had no valid device yet.
type Binding struct {
	Session        auth.Session
	NewDeviceToken string
}

// SessionByToken finds the device for a cookie token and its active member.
// It returns ErrNotFound for unknown or expired devices and for devices
// without an active member.
func (s *Store) SessionByToken(ctx context.Context, token string) (auth.Session, error) {
	var sess auth.Session
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, m.id, m.family_id
		FROM devices d
		JOIN members m ON m.id = d.active_member_id
		WHERE d.token_hash = $1 AND d.created_at > $2`,
		auth.HashToken(token), deviceCutoff(),
	).Scan(&sess.DeviceID, &sess.MemberID, &sess.FamilyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.Session{}, ErrNotFound
	}
	return sess, err
}

// CreateFamily creates a family, its owner and its invite link, binds the
// owner to the device and makes the owner active, all in one transaction.
// deviceToken may be empty or unknown; then a new device is created.
func (s *Store) CreateFamily(ctx context.Context, deviceToken, familyName, memberName, relationship string) (Binding, error) {
	var b Binding
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		deviceID, newToken, err := deviceForToken(ctx, tx, deviceToken)
		if err != nil {
			return err
		}

		var familyID, memberID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO families (name) VALUES ($1) RETURNING id`, familyName,
		).Scan(&familyID); err != nil {
			return fmt.Errorf("insert family: %w", err)
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO members (family_id, name, relationship, is_owner) VALUES ($1, $2, $3, true) RETURNING id`,
			familyID, memberName, relationship,
		).Scan(&memberID); err != nil {
			return fmt.Errorf("insert owner: %w", err)
		}

		inviteToken, err := auth.NewToken()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO invitations (token, family_id, created_by) VALUES ($1, $2, $3)`,
			inviteToken, familyID, memberID,
		); err != nil {
			return fmt.Errorf("insert invitation: %w", err)
		}

		if err := bindAndActivate(ctx, tx, deviceID, memberID); err != nil {
			return err
		}
		b = Binding{
			Session:        auth.Session{DeviceID: deviceID, MemberID: memberID, FamilyID: familyID},
			NewDeviceToken: newToken,
		}
		return nil
	})
	return b, err
}

// JoinFamily adds a member to the family behind inviteToken, binds the member
// to the device and makes the member active, all in one transaction.
// It returns ErrNotFound for an unknown invite and ErrFamilyFull when the
// family already has MaxMembers members.
func (s *Store) JoinFamily(ctx context.Context, deviceToken, inviteToken, memberName, relationship string) (Binding, error) {
	var b Binding
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var familyID string
		err := tx.QueryRow(ctx, `SELECT family_id FROM invitations WHERE token = $1`, inviteToken).Scan(&familyID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("find invitation: %w", err)
		}

		// Lock the family row so concurrent joins cannot exceed the limit.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM families WHERE id = $1 FOR UPDATE`, familyID); err != nil {
			return fmt.Errorf("lock family: %w", err)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM members WHERE family_id = $1`, familyID).Scan(&count); err != nil {
			return fmt.Errorf("count members: %w", err)
		}
		if count >= MaxMembers {
			return ErrFamilyFull
		}

		deviceID, newToken, err := deviceForToken(ctx, tx, deviceToken)
		if err != nil {
			return err
		}
		var memberID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO members (family_id, name, relationship) VALUES ($1, $2, $3) RETURNING id`,
			familyID, memberName, relationship,
		).Scan(&memberID); err != nil {
			return fmt.Errorf("insert member: %w", err)
		}
		if err := bindAndActivate(ctx, tx, deviceID, memberID); err != nil {
			return err
		}
		b = Binding{
			Session:        auth.Session{DeviceID: deviceID, MemberID: memberID, FamilyID: familyID},
			NewDeviceToken: newToken,
		}
		return nil
	})
	return b, err
}

// InviteByToken returns the family name and inviter for an invite link.
func (s *Store) InviteByToken(ctx context.Context, token string) (Invite, error) {
	var inv Invite
	err := s.pool.QueryRow(ctx, `
		SELECT f.name, m.name
		FROM invitations i
		JOIN families f ON f.id = i.family_id
		JOIN members m ON m.id = i.created_by
		WHERE i.token = $1`, token,
	).Scan(&inv.FamilyName, &inv.InvitedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, ErrNotFound
	}
	return inv, err
}

// Me returns the active member, their family and all members of the device.
func (s *Store) Me(ctx context.Context, sess auth.Session) (Me, error) {
	var me Me
	err := s.pool.QueryRow(ctx, `
		SELECT m.id, m.name, m.relationship, m.is_owner, f.id, f.name
		FROM members m
		JOIN families f ON f.id = m.family_id
		WHERE m.id = $1`, sess.MemberID,
	).Scan(&me.Member.ID, &me.Member.Name, &me.Member.Relationship, &me.Member.IsOwner, &me.Family.ID, &me.Family.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Me{}, ErrNotFound
	}
	if err != nil {
		return Me{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT m.id, m.name, m.relationship, f.name
		FROM device_members dm
		JOIN members m ON m.id = dm.member_id
		JOIN families f ON f.id = m.family_id
		WHERE dm.device_id = $1
		ORDER BY m.created_at, m.id`, sess.DeviceID)
	if err != nil {
		return Me{}, err
	}
	me.DeviceMembers, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (DeviceMember, error) {
		var dm DeviceMember
		err := row.Scan(&dm.ID, &dm.Name, &dm.Relationship, &dm.FamilyName)
		return dm, err
	})
	return me, err
}

// FamilyByID returns a family, its members in joining order and its invite.
// Callers must pass the family ID from the session.
func (s *Store) FamilyByID(ctx context.Context, familyID string) (Family, error) {
	var fam Family
	err := s.pool.QueryRow(ctx, `
		SELECT f.id, f.name, i.token
		FROM families f
		JOIN invitations i ON i.family_id = f.id
		WHERE f.id = $1`, familyID,
	).Scan(&fam.Family.ID, &fam.Family.Name, &fam.InviteToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return Family{}, ErrNotFound
	}
	if err != nil {
		return Family{}, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, name, relationship, is_owner
		FROM members
		WHERE family_id = $1
		ORDER BY created_at, id`, familyID)
	if err != nil {
		return Family{}, err
	}
	fam.Members, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (Member, error) {
		var m Member
		err := row.Scan(&m.ID, &m.Name, &m.Relationship, &m.IsOwner)
		return m, err
	})
	return fam, err
}

// SwitchActiveMember makes memberID the device's active member. It returns
// ErrNotDeviceMember unless the member was created on this device.
func (s *Store) SwitchActiveMember(ctx context.Context, deviceID, memberID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE devices SET active_member_id = $2
		WHERE id = $1
		  AND EXISTS (SELECT 1 FROM device_members WHERE device_id = $1 AND member_id = $2)`,
		deviceID, memberID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotDeviceMember
	}
	return nil
}

// deviceForToken returns the device for a valid cookie token, or creates a
// new device. newToken is set only when a device was created.
func deviceForToken(ctx context.Context, tx pgx.Tx, token string) (deviceID, newToken string, err error) {
	if token != "" {
		err := tx.QueryRow(ctx,
			`SELECT id FROM devices WHERE token_hash = $1 AND created_at > $2`,
			auth.HashToken(token), deviceCutoff(),
		).Scan(&deviceID)
		if err == nil {
			return deviceID, "", nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", "", fmt.Errorf("find device: %w", err)
		}
	}

	newToken, err = auth.NewToken()
	if err != nil {
		return "", "", err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO devices (token_hash) VALUES ($1) RETURNING id`, auth.HashToken(newToken),
	).Scan(&deviceID); err != nil {
		return "", "", fmt.Errorf("insert device: %w", err)
	}
	return deviceID, newToken, nil
}

func bindAndActivate(ctx context.Context, tx pgx.Tx, deviceID, memberID string) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO device_members (device_id, member_id) VALUES ($1, $2)`, deviceID, memberID,
	); err != nil {
		return fmt.Errorf("bind member to device: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE devices SET active_member_id = $2 WHERE id = $1`, deviceID, memberID,
	); err != nil {
		return fmt.Errorf("activate member: %w", err)
	}
	return nil
}

// deviceCutoff is the oldest creation time of a device that is still valid.
func deviceCutoff() time.Time {
	return time.Now().Add(-auth.DeviceTTL)
}
