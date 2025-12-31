package db

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mrshanahan/simple-password-service/internal/crypto"
)

type PassdDb struct {
	db  *sql.DB
	key crypto.PassdKey
}

var (
	//go:embed files/create_passwords_table.sql
	CreatePasswordTableSql string
	KeySize                int   = 32
	ErrConflict            error = fmt.Errorf("password with id already exists")
)

func Open(dbPath string, key crypto.PassdKey) (*PassdDb, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, fmt.Errorf("DB reference still nil despite no error")
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(CreatePasswordTableSql)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to setup passwords table: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to setup passwords table: %w", err)
	}

	return &PassdDb{db, key}, nil
}

func (passddb *PassdDb) Close() error {
	return passddb.db.Close()
}

func (passddb *PassdDb) LoadHash(id string) ([]byte, error) {
	stmt, err := passddb.db.Prepare("SELECT password_enc FROM passwords WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var passwordEnc []byte
	row := stmt.QueryRow(id)
	if err := row.Scan(&passwordEnc); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to load entry: %w", err)
	}

	passwordDec, err := passddb.key.Decrypt(passwordEnc)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}

	passwordHash, err := crypto.Hash(passwordDec)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	return passwordHash, nil
}

func (passddb *PassdDb) CreatePassword(id string, password string) error {
	stmt, err := passddb.db.Prepare("INSERT INTO passwords (id, password_enc) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	ciphertext, err := passddb.key.Encrypt([]byte(password))
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}
	if _, err := stmt.Exec(id, ciphertext); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (passddb *PassdDb) UpsertPassword(id string, password string) error {
	stmt, err := passddb.db.Prepare("INSERT INTO passwords (id, password_enc) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET password_enc = excluded.password_enc")
	if err != nil {
		return fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	ciphertext, err := passddb.key.Encrypt([]byte(password))
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}
	if _, err := stmt.Exec(id, ciphertext); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

func (passddb *PassdDb) DeleteEntry(id string) (bool, error) {
	stmt, err := passddb.db.Prepare("DELETE FROM passwords WHERE id = ?")
	if err != nil {
		return false, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(id)
	if err != nil {
		return false, fmt.Errorf("failed to delete entry: %w", err)
	}

	// We're ignoring the error here b/c we know our driver supports RowsAffected()
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
}

func (passddb *PassdDb) GetPassword(id string) ([]byte, error) {
	stmt, err := passddb.db.Prepare("SELECT password_enc FROM passwords WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	var ciphertext []byte
	if err := stmt.QueryRow(id).Scan(&ciphertext); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	plaintext, err := passddb.key.Decrypt(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt password: %w", err)
	}

	return plaintext, nil
}

func (passddb *PassdDb) ListIds() ([]string, error) {
	stmt, err := passddb.db.Prepare("SELECT id FROM passwords")
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query()
	if err != nil {
		return nil, fmt.Errorf("failed to eecute query: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
