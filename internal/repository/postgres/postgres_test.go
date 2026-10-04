package postgres

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/katshuuu/pr-pigeon/internal/domain"
)

func newMock(t *testing.T) (*Repository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return New(db), mock
}

func TestMigrate(t *testing.T) {
	r, mock := newMock(t)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS subscriptions").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := r.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestUpsert(t *testing.T) {
	r, mock := newMock(t)
	now := time.Now()
	mock.ExpectExec("INSERT INTO subscriptions").
		WithArgs(int64(1), "octocat", now, now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO subscriptions").WillReturnError(errors.New("conn reset"))

	sub := domain.Subscription{ChatID: 1, GitHubLogin: "octocat", CreatedAt: now, UpdatedAt: now}
	if err := r.Upsert(context.Background(), sub); err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(context.Background(), sub); err == nil {
		t.Fatal("expected error")
	}
}

func TestGet(t *testing.T) {
	r, mock := newMock(t)
	now := time.Now()
	cols := []string{"chat_id", "github_login", "created_at", "updated_at"}

	mock.ExpectQuery("SELECT chat_id, github_login").WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(1), "octocat", now, now))
	mock.ExpectQuery("SELECT chat_id, github_login").WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows(cols))
	mock.ExpectQuery("SELECT chat_id, github_login").WillReturnError(errors.New("timeout"))

	ctx := context.Background()
	if s, err := r.Get(ctx, 1); err != nil || s.GitHubLogin != "octocat" {
		t.Errorf("Get(1) = %+v, %v", s, err)
	}
	if _, err := r.Get(ctx, 2); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get(2) err = %v", err)
	}
	if _, err := r.Get(ctx, 3); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get(3) err = %v", err)
	}
}

func TestDelete(t *testing.T) {
	r, mock := newMock(t)
	mock.ExpectExec("DELETE FROM subscriptions").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM subscriptions").WithArgs(int64(2)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM subscriptions").WillReturnError(errors.New("boom"))

	ctx := context.Background()
	if err := r.Delete(ctx, 1); err != nil {
		t.Error(err)
	}
	if err := r.Delete(ctx, 2); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v", err)
	}
	if err := r.Delete(ctx, 3); err == nil {
		t.Error("expected error")
	}
}

func TestChatIDsByLogin(t *testing.T) {
	r, mock := newMock(t)
	mock.ExpectQuery("SELECT chat_id FROM subscriptions").WithArgs("OctoCat").
		WillReturnRows(sqlmock.NewRows([]string{"chat_id"}).AddRow(int64(10)).AddRow(int64(20)))
	mock.ExpectQuery("SELECT chat_id FROM subscriptions").WillReturnError(errors.New("boom"))

	ids, err := r.ChatIDsByLogin(context.Background(), "OctoCat")
	if err != nil || !reflect.DeepEqual(ids, []int64{10, 20}) {
		t.Errorf("ids = %v, err = %v", ids, err)
	}
	if _, err := r.ChatIDsByLogin(context.Background(), "x"); err == nil {
		t.Error("expected error")
	}
}
