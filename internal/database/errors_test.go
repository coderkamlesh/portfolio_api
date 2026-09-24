package database

import (
	"errors"
	"fmt"
	"testing"

	turso "turso.tech/database/tursogo-serverless"
)

func TestIsUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "server sent the unique code",
			err:  &turso.Error{Code: "SQLITE_CONSTRAINT_UNIQUE", Message: "constraint failed"},
			want: true,
		},
		{
			name: "server sent only the message",
			err:  &turso.Error{Message: "UNIQUE constraint failed: skills.category_id, skills.skill_name"},
			want: true,
		},
		{
			name: "primary key clash",
			err:  &turso.Error{Code: "SQLITE_CONSTRAINT_PRIMARYKEY", Message: "constraint failed"},
			want: true,
		},
		{
			name: "foreign key failure is not a unique violation",
			err:  &turso.Error{Code: "SQLITE_CONSTRAINT_FOREIGNKEY", Message: "FOREIGN KEY constraint failed"},
			want: false,
		},
		{
			name: "wrapped driver error is still detected",
			err:  fmt.Errorf("skill: create category: %w", &turso.Error{Message: "UNIQUE constraint failed: skill_categories.name"}),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection reset"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsUniqueViolation(tc.err); got != tc.want {
				t.Fatalf("IsUniqueViolation(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
