// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Localization repository (ADR-0063): the persistence behind the beta-tester
// translation tool. Two tables from migration 097 — localization_locales
// (the locale registry) and localization_translations (one row per
// locale+key, the currently accepted submission).
//
// The key list is deliberately NOT stored here. It is derived from the English
// catalog in the repo, so a key that exists in the DB but not in the catalog is
// dead weight the export silently drops, and a new catalog key is simply a key
// nobody has translated yet. The repo validates key SHAPE only.

// ErrLocaleNotApproved is returned when a translation targets a locale that is
// not in the approved state (unknown or still a request). Callers map it to 404.
var ErrLocaleNotApproved = errors.New("locale not approved")

// LocalizationRepository handles localization contribution persistence.
type LocalizationRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewLocalizationRepository creates a new localization repository.
func NewLocalizationRepository(db *pgxpool.Pool, logger *zap.Logger) *LocalizationRepository {
	return &LocalizationRepository{db: db, logger: logger}
}

// Locale is one row of localization_locales.
type Locale struct {
	Code        string `json:"code"`
	EnglishName string `json:"english_name"`
	NativeName  string `json:"native_name"`
	Status      string `json:"status"` // 'requested' | 'approved'
	RequestedBy string `json:"requested_by,omitempty"`
}

// Translation is one row of localization_translations.
type Translation struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Status      string  `json:"status"` // 'pending' | 'approved' | 'rejected'
	SubmittedBy string  `json:"submitted_by,omitempty"`
	ReviewedBy  string  `json:"reviewed_by,omitempty"`
	ReviewNote  *string `json:"review_note,omitempty"`
	UpdatedAt   string  `json:"updated_at"`
}

// Key shape validation. The catalog convention (docs/frontend/I18N.md) is a
// namespace file name, then at most three camelCase/dot levels: 'common.save',
// 'overlayEditor.appearance.title'. Placeholder syntax inside VALUES is
// validated separately at the handler.
var localeCodePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z]{2,4})?$`)
var keyPattern = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*){1,3}$`)

// ValidLocaleCode reports whether code is a BCP 47-ish locale code ('de',
// 'pt-BR'). Full validation of every BCP 47 subtag is not worth it here: the
// code becomes a directory name at export time, so the shape only needs to be
// filesystem-safe and unambiguous.
func ValidLocaleCode(code string) bool {
	return len(code) <= 16 && localeCodePattern.MatchString(code)
}

// ValidKey reports whether key looks like a catalog key (dot-separated, 2–4
// segments, each starting with a letter). It does not check that the key exists
// in the catalog — that check happens against the repo, not the DB.
func ValidKey(key string) bool {
	return len(key) <= 200 && keyPattern.MatchString(key)
}

// RequestLocale inserts a locale request, or revives an existing approved
// locale's metadata (names can be corrected by anyone). A duplicate request by
// another contributor is a no-op success — requests are not competitive.
func (r *LocalizationRepository) RequestLocale(ctx context.Context, code, englishName, nativeName, requestedBy string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO localization_locales (code, english_name, native_name, status, requested_by)
		 VALUES ($1, $2, $3, 'requested', $4)
		 ON CONFLICT (code) DO NOTHING`,
		code, englishName, nativeName, requestedBy)
	if err != nil {
		r.logger.Error("Failed to insert locale request",
			zap.String("code", code), zap.Error(err))
		return fmt.Errorf("failed to insert locale request: %w", err)
	}
	return nil
}

// ListApprovedLocales returns the locales a contributor can translate into.
func (r *LocalizationRepository) ListApprovedLocales(ctx context.Context) ([]Locale, error) {
	rows, err := r.db.Query(ctx,
		`SELECT code, english_name, native_name FROM localization_locales
		 WHERE status = 'approved' ORDER BY english_name`)
	if err != nil {
		r.logger.Error("Failed to list approved locales", zap.Error(err))
		return nil, fmt.Errorf("failed to list approved locales: %w", err)
	}
	defer rows.Close()

	var locales []Locale
	for rows.Next() {
		var l Locale
		if err := rows.Scan(&l.Code, &l.EnglishName, &l.NativeName); err != nil {
			return nil, fmt.Errorf("failed to scan locale: %w", err)
		}
		locales = append(locales, l)
	}
	return locales, rows.Err()
}

// ListRequestedLocales returns the locales awaiting admin approval.
func (r *LocalizationRepository) ListRequestedLocales(ctx context.Context) ([]Locale, error) {
	rows, err := r.db.Query(ctx,
		`SELECT code, english_name, native_name,
		        COALESCE(requested_by::text, '') AS requested_by
		 FROM localization_locales
		 WHERE status = 'requested' ORDER BY created_at`)
	if err != nil {
		r.logger.Error("Failed to list requested locales", zap.Error(err))
		return nil, fmt.Errorf("failed to list requested locales: %w", err)
	}
	defer rows.Close()

	var locales []Locale
	for rows.Next() {
		var l Locale
		if err := rows.Scan(&l.Code, &l.EnglishName, &l.NativeName, &l.RequestedBy); err != nil {
			return nil, fmt.Errorf("failed to scan locale request: %w", err)
		}
		locales = append(locales, l)
	}
	return locales, rows.Err()
}

// ApproveLocaleRequest moves a locale request to approved, or creates an
// approved locale outright (the admin console seeds 'de' this way). An already
// approved locale is a no-op.
func (r *LocalizationRepository) ApproveLocaleRequest(ctx context.Context, code, reviewedBy string) error {
	// Create as approved if absent; the caller already validated the code shape.
	_, err := r.db.Exec(ctx,
		`INSERT INTO localization_locales (code, english_name, native_name, status, reviewed_by)
		 VALUES ($1, '', '', 'approved', $2)
		 ON CONFLICT (code) DO NOTHING`,
		code, reviewedBy)
	if err != nil {
		r.logger.Error("Failed to seed approved locale", zap.String("code", code), zap.Error(err))
		return fmt.Errorf("failed to seed approved locale: %w", err)
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE localization_locales SET status = 'approved', reviewed_by = $2, updated_at = NOW()
		 WHERE code = $1 AND status = 'requested'`,
		code, reviewedBy)
	if err != nil {
		r.logger.Error("Failed to approve locale", zap.String("code", code), zap.Error(err))
		return fmt.Errorf("failed to approve locale: %w", err)
	}
	// A no-op update (already approved) is fine; approval is idempotent.
	_ = tag
	return nil
}

// RejectLocaleRequest deletes a requested locale. Rejection carries no history
// worth keeping — the contributor can simply re-request with better names.
func (r *LocalizationRepository) RejectLocaleRequest(ctx context.Context, code string) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM localization_locales WHERE code = $1 AND status = 'requested'`,
		code)
	if err != nil {
		r.logger.Error("Failed to reject locale request", zap.String("code", code), zap.Error(err))
		return fmt.Errorf("failed to reject locale request: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("locale request not found: %s", code)
	}
	return nil
}

// UpsertTranslation saves one (locale, key) submission, overwriting whatever
// row was there and resetting it to 'pending'. Only approved locales accept
// submissions — a requested or unknown locale returns ErrLocaleNotApproved.
func (r *LocalizationRepository) UpsertTranslation(ctx context.Context, locale, key, value, submittedBy string) error {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM localization_locales WHERE code = $1`, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLocaleNotApproved
		}
		return fmt.Errorf("failed to read locale: %w", err)
	}
	if status != "approved" {
		return ErrLocaleNotApproved
	}

	_, err = r.db.Exec(ctx,
		`INSERT INTO localization_translations (locale, key, value, status, submitted_by)
		 VALUES ($1, $2, $3, 'pending', $4)
		 ON CONFLICT (locale, key) DO UPDATE SET
		   value = EXCLUDED.value,
		   status = 'pending',
		   submitted_by = EXCLUDED.submitted_by,
		   reviewed_by = NULL,
		   review_note = NULL,
		   updated_at = NOW()`,
		locale, key, value, submittedBy)
	if err != nil {
		r.logger.Error("Failed to upsert translation",
			zap.String("locale", locale), zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to upsert translation: %w", err)
	}
	return nil
}

// ListTranslations returns the caller's rows for a locale. Statuses:
// 'pending' (awaiting review), 'approved', 'rejected' (with review_note —
// the contributor revises and resubmits, overwriting the row).
func (r *LocalizationRepository) ListTranslations(ctx context.Context, locale string) ([]Translation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT key, value, status, COALESCE(submitted_by::text, ''), review_note,
		        to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		 FROM localization_translations
		 WHERE locale = $1 AND submitted_by = $2::uuid
		 ORDER BY key`,
		locale)
	if err != nil {
		r.logger.Error("Failed to list translations", zap.String("locale", locale), zap.Error(err))
		return nil, fmt.Errorf("failed to list translations: %w", err)
	}
	defer rows.Close()
	return scanTranslations(rows)
}

// scanTranslations drains a translation result set.
func scanTranslations(rows pgx.Rows) ([]Translation, error) {
	var out []Translation
	for rows.Next() {
		var t Translation
		if err := rows.Scan(&t.Key, &t.Value, &t.Status, &t.SubmittedBy, &t.ReviewNote, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan translation: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListPendingTranslations returns the review queue: every pending row across
// approved locales, oldest first so the earliest contributor is reviewed first.
func (r *LocalizationRepository) ListPendingTranslations(ctx context.Context) ([]Translation, error) {
	rows, err := r.db.Query(ctx,
		`SELECT t.key, t.value, t.status, COALESCE(t.submitted_by::text, ''), t.review_note,
		        to_char(t.updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		 FROM localization_translations t
		 JOIN localization_locales l ON l.code = t.locale
		 WHERE t.status = 'pending' AND l.status = 'approved'
		 ORDER BY t.updated_at`)
	if err != nil {
		r.logger.Error("Failed to list pending translations", zap.Error(err))
		return nil, fmt.Errorf("failed to list pending translations: %w", err)
	}
	defer rows.Close()
	return scanTranslations(rows)
}

// ReviewTranslation sets a pending row to 'approved' or 'rejected'. Only
// pending rows are actionable; reviewing an already-reviewed row is a no-op
// (approving twice is harmless, re-rejecting after a resubmission works
// because the resubmission reset the row to 'pending').
func (r *LocalizationRepository) ReviewTranslation(ctx context.Context, locale, key, decision, reviewNote, reviewedBy string) error {
	if decision != "approved" && decision != "rejected" {
		return fmt.Errorf("invalid decision: %s", decision)
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE localization_translations
		 SET status = $3, reviewed_by = $4, review_note = $5, updated_at = NOW()
		 WHERE locale = $1 AND key = $2 AND status = 'pending'`,
		locale, key, decision, reviewedBy, reviewNote)
	if err != nil {
		r.logger.Error("Failed to review translation",
			zap.String("locale", locale), zap.String("key", key), zap.Error(err))
		return fmt.Errorf("failed to review translation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("pending translation not found: %s/%s", locale, key)
	}
	return nil
}

// ExportLocale returns every approved row for a locale. The export endpoint
// wraps this; scripts/generate-locale-catalog.mjs turns the JSON into catalog
// files. Keys absent from the English catalog are still returned — the
// generator skips them, since the catalog is the key source of truth.
func (r *LocalizationRepository) ExportLocale(ctx context.Context, locale string) ([]Translation, error) {
	var status string
	err := r.db.QueryRow(ctx,
		`SELECT status FROM localization_locales WHERE code = $1`, locale).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLocaleNotApproved
		}
		return nil, fmt.Errorf("failed to read locale: %w", err)
	}
	if status != "approved" {
		return nil, ErrLocaleNotApproved
	}

	rows, err := r.db.Query(ctx,
		`SELECT key, value, status, COALESCE(submitted_by::text, ''), review_note,
		        to_char(updated_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		 FROM localization_translations
		 WHERE locale = $1 AND status = 'approved'
		 ORDER BY key`,
		locale)
	if err != nil {
		r.logger.Error("Failed to export locale", zap.String("locale", locale), zap.Error(err))
		return nil, fmt.Errorf("failed to export locale: %w", err)
	}
	defer rows.Close()
	return scanTranslations(rows)
}

// LocaleProgress returns per-locale counts for the admin review surface.
type LocaleProgress struct {
	Code     string `json:"code"`
	Pending  int    `json:"pending"`
	Approved int    `json:"approved"`
	Rejected int    `json:"rejected"`
}

// ListLocaleProgress returns pending/approved/rejected counts per locale,
// ordered by pending descending (the review queue order of interest).
func (r *LocalizationRepository) ListLocaleProgress(ctx context.Context) ([]LocaleProgress, error) {
	rows, err := r.db.Query(ctx,
		`SELECT l.code,
		        COUNT(*) FILTER (WHERE t.status = 'pending'),
		        COUNT(*) FILTER (WHERE t.status = 'approved'),
		        COUNT(*) FILTER (WHERE t.status = 'rejected')
		 FROM localization_locales l
		 LEFT JOIN localization_translations t ON t.locale = l.code
		 WHERE l.status = 'approved'
		 GROUP BY l.code
		 ORDER BY 2 DESC, l.code`)
	if err != nil {
		r.logger.Error("Failed to list locale progress", zap.Error(err))
		return nil, fmt.Errorf("failed to list locale progress: %w", err)
	}
	defer rows.Close()

	var out []LocaleProgress
	for rows.Next() {
		var p LocaleProgress
		if err := rows.Scan(&p.Code, &p.Pending, &p.Approved, &p.Rejected); err != nil {
			return nil, fmt.Errorf("failed to scan locale progress: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
