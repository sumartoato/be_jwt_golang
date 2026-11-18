package db

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// DemoPassword is the password of every demo user created by SeedDemoUsers.
const DemoPassword = "Medika123"

// DemoUsers maps each demo email to the role it gets.
var DemoUsers = []struct{ Email, Role string }{
	{"admin@medika.com", "Admin"},
	{"cs@medika.com", "Pendaftaran"},
	{"perawat@medika.com", "Perawat"},
}

// SeedDemoUsers creates the demo users that do not exist yet. Existing users
// are left alone, so a changed password or role survives restarts.
// Only call it when SEED_DEMO_USERS=true: the password is public.
func SeedDemoUsers(ctx context.Context, pool *pgxpool.Pool, bcryptCost int) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), bcryptCost)
	if err != nil {
		return err
	}
	for _, u := range DemoUsers {
		tag, err := pool.Exec(ctx,
			`INSERT INTO users (id, email, password_hash, role_id)
			 SELECT $1, $2, $3, (SELECT id FROM roles WHERE name = $4)
			 ON CONFLICT (email) DO NOTHING`,
			uuid.NewString(), u.Email, string(hash), u.Role)
		if err != nil {
			return fmt.Errorf("seed %s: %w", u.Email, err)
		}
		if tag.RowsAffected() == 1 {
			log.Printf("created demo user %s (role %s)", u.Email, u.Role)
		}
	}
	return nil
}
