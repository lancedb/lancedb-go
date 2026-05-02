// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package tests

import (
	"context"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/memory"

	"github.com/lancedb/lancedb-go/pkg/contracts"
	"github.com/lancedb/lancedb-go/pkg/internal"
	"github.com/lancedb/lancedb-go/pkg/lancedb"
)

// TestTimeTravel exercises the version-history surface end-to-end.
// Each sub-test seeds a fresh table so cases stay independent. The
// scenario mirrors the upstream Python tests in lancedb's
// test_table.py::test_restore + test_restore_with_tags.
func TestTimeTravel(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lancedb_test_time_travel_")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	conn, err := lancedb.Connect(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	arrowSchema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	schema, err := internal.NewSchema(arrowSchema)
	if err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}

	pool := memory.NewGoAllocator()

	// seed creates a fresh table named `name`, adds two rows, and
	// returns it asserted to the time-travel capability extension.
	// After seed: version >= 2 (create + 1 add).
	seed := func(t *testing.T, name string) (contracts.ITable, contracts.ITableTimeTravel) {
		t.Helper()
		table, err := conn.CreateTable(context.Background(), name, schema)
		if err != nil {
			t.Fatalf("create table: %v", err)
		}
		t.Cleanup(func() { _ = table.Close() })

		rec := buildRecord(t, pool, arrowSchema,
			[]int32{1, 2},
			[]string{"Alice", "Bob"},
			[]float64{10, 20})
		defer rec.Release()
		if err := table.Add(context.Background(), rec, nil); err != nil {
			t.Fatalf("seed add: %v", err)
		}
		tt, ok := table.(contracts.ITableTimeTravel)
		if !ok {
			t.Fatalf("table does not implement contracts.ITableTimeTravel")
		}
		return table, tt
	}

	// addOne appends a single row with the given id+name. Used to bump
	// the version counter so checkout/restore can target known states.
	addOne := func(t *testing.T, table contracts.ITable, id int32, name string) {
		t.Helper()
		rec := buildRecord(t, pool, arrowSchema,
			[]int32{id},
			[]string{name},
			[]float64{float64(id)})
		defer rec.Release()
		if err := table.Add(context.Background(), rec, nil); err != nil {
			t.Fatalf("add row: %v", err)
		}
	}

	t.Run("ListVersions", func(t *testing.T) {
		table, tt := seed(t, "tt_list_versions")
		_ = table

		versions, err := tt.ListVersions(context.Background())
		if err != nil {
			t.Fatalf("ListVersions: %v", err)
		}
		if len(versions) < 2 {
			t.Fatalf("expected at least 2 versions after create+add, got %d", len(versions))
		}
		// Versions must be strictly increasing — that's a property the
		// Lance manifest log enforces.
		for i := 1; i < len(versions); i++ {
			if versions[i].Version <= versions[i-1].Version {
				t.Fatalf("versions not strictly increasing: %v then %v",
					versions[i-1].Version, versions[i].Version)
			}
		}
		// Timestamps must parse as non-zero UTC times.
		for i, v := range versions {
			if v.Timestamp.IsZero() {
				t.Fatalf("version %d has zero timestamp", i)
			}
		}
	})

	t.Run("CheckoutAndCheckoutLatest", func(t *testing.T) {
		table, tt := seed(t, "tt_checkout_latest")

		got, err := table.Version(context.Background())
		if err != nil {
			t.Fatalf("Version: %v", err)
		}
		latest := uint64(got)

		// Pin to v1 (create-only state — should have only the rows
		// from create_table, but our seed uses CreateTable with no
		// data so v1 is empty; either way Count must be < 2).
		if err := tt.Checkout(context.Background(), 1); err != nil {
			t.Fatalf("Checkout(1): %v", err)
		}
		c1, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count after checkout(1): %v", err)
		}
		if c1 >= 2 {
			t.Fatalf("expected fewer than 2 rows at v1, got %d", c1)
		}

		// Drop pin and confirm we see latest's row count again.
		if err := tt.CheckoutLatest(context.Background()); err != nil {
			t.Fatalf("CheckoutLatest: %v", err)
		}
		cN, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count after CheckoutLatest: %v", err)
		}
		if cN != 2 {
			t.Fatalf("expected 2 rows at latest after CheckoutLatest, got %d", cN)
		}

		// Sanity: Version() must report the same latest we observed
		// before pinning.
		got2, err := table.Version(context.Background())
		if err != nil {
			t.Fatalf("Version after CheckoutLatest: %v", err)
		}
		if uint64(got2) != latest {
			t.Fatalf("version drifted across pin/unpin: was %d, now %d", latest, got2)
		}
	})

	t.Run("RestorePromotesCheckedOutVersion", func(t *testing.T) {
		// Build v1 (create) -> v2 (add Alice/Bob) -> v3 (add Charlie).
		// Pin to v2, restore -> a new v4 manifest carrying v2's data.
		table, tt := seed(t, "tt_restore_promote")
		addOne(t, table, 3, "Charlie")

		count3, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count v3: %v", err)
		}
		if count3 != 3 {
			t.Fatalf("expected 3 rows at v3, got %d", count3)
		}

		if err := tt.Checkout(context.Background(), 2); err != nil {
			t.Fatalf("Checkout(2): %v", err)
		}
		// Writes must be rejected while pinned. We use TagCreate as a
		// pure-metadata write surrogate — but Tag operations don't
		// produce dataset commits, so they actually succeed even on
		// checked-out tables. Use a real write (Add) instead.
		rec := buildRecord(t, pool, arrowSchema,
			[]int32{99}, []string{"X"}, []float64{99})
		writeErr := table.Add(context.Background(), rec, nil)
		rec.Release()
		if writeErr == nil {
			t.Fatalf("expected Add to fail on checked-out table")
		}

		if err := tt.Restore(context.Background()); err != nil {
			t.Fatalf("Restore: %v", err)
		}

		count4, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count after Restore: %v", err)
		}
		if count4 != 2 {
			t.Fatalf("expected 2 rows after restoring v2, got %d", count4)
		}

		// History must have grown — the Restore is a new commit.
		versions, err := tt.ListVersions(context.Background())
		if err != nil {
			t.Fatalf("ListVersions after Restore: %v", err)
		}
		if len(versions) < 4 {
			t.Fatalf("expected >= 4 versions after Restore, got %d", len(versions))
		}
	})

	t.Run("RestoreWithoutCheckoutFails", func(t *testing.T) {
		_, tt := seed(t, "tt_restore_unpinned")
		// Without a prior checkout the table has nothing to promote.
		if err := tt.Restore(context.Background()); err == nil {
			t.Fatalf("expected Restore on unpinned table to fail")
		}
	})

	t.Run("CheckoutUnknownVersionFails", func(t *testing.T) {
		_, tt := seed(t, "tt_checkout_unknown")
		if err := tt.Checkout(context.Background(), 9999); err == nil {
			t.Fatalf("expected Checkout(9999) to fail")
		}
	})

	t.Run("TagsCRUDFullCycle", func(t *testing.T) {
		table, tt := seed(t, "tt_tags_crud")
		_ = table

		// Create two tags pointing at v2 (the post-seed version).
		if err := tt.TagCreate(context.Background(), "rollback-safe", 2); err != nil {
			t.Fatalf("TagCreate rollback-safe: %v", err)
		}
		if err := tt.TagCreate(context.Background(), "release-2026-05", 2); err != nil {
			t.Fatalf("TagCreate release-2026-05: %v", err)
		}

		tags, err := tt.TagList(context.Background())
		if err != nil {
			t.Fatalf("TagList: %v", err)
		}
		if len(tags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(tags))
		}
		if tags["rollback-safe"].Version != 2 {
			t.Fatalf("rollback-safe pinned to wrong version: %d", tags["rollback-safe"].Version)
		}
		if tags["rollback-safe"].ManifestSize == 0 {
			t.Fatalf("rollback-safe has zero manifest_size — backend should report > 0")
		}

		// Resolve via TagGetVersion.
		v, err := tt.TagGetVersion(context.Background(), "rollback-safe")
		if err != nil {
			t.Fatalf("TagGetVersion: %v", err)
		}
		if v != 2 {
			t.Fatalf("TagGetVersion returned %d, want 2", v)
		}

		// Bump the table to v3 and move rollback-safe forward.
		addOne(t, table, 3, "Charlie")
		if err := tt.TagUpdate(context.Background(), "rollback-safe", 3); err != nil {
			t.Fatalf("TagUpdate: %v", err)
		}
		v, err = tt.TagGetVersion(context.Background(), "rollback-safe")
		if err != nil {
			t.Fatalf("TagGetVersion after update: %v", err)
		}
		if v != 3 {
			t.Fatalf("TagUpdate did not move tag: got %d, want 3", v)
		}

		// Delete one tag and verify the survivor.
		if err := tt.TagDelete(context.Background(), "release-2026-05"); err != nil {
			t.Fatalf("TagDelete: %v", err)
		}
		tags, err = tt.TagList(context.Background())
		if err != nil {
			t.Fatalf("TagList after delete: %v", err)
		}
		if len(tags) != 1 {
			t.Fatalf("expected 1 tag after delete, got %d", len(tags))
		}
		if _, ok := tags["release-2026-05"]; ok {
			t.Fatalf("release-2026-05 should be gone")
		}
	})

	t.Run("CheckoutTagThenRestore", func(t *testing.T) {
		table, tt := seed(t, "tt_checkout_tag_restore")
		// Tag v2 as the safe rollback point, then add a v3 we want to
		// undo.
		if err := tt.TagCreate(context.Background(), "rollback", 2); err != nil {
			t.Fatalf("TagCreate: %v", err)
		}
		addOne(t, table, 3, "Charlie")
		if err := tt.CheckoutTag(context.Background(), "rollback"); err != nil {
			t.Fatalf("CheckoutTag: %v", err)
		}
		if err := tt.Restore(context.Background()); err != nil {
			t.Fatalf("Restore after CheckoutTag: %v", err)
		}
		count, err := table.Count(context.Background())
		if err != nil {
			t.Fatalf("Count after Restore: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2 rows after restoring tag rollback, got %d", count)
		}
	})

	t.Run("TagErrorPaths", func(t *testing.T) {
		_, tt := seed(t, "tt_tag_errors")

		if _, err := tt.TagGetVersion(context.Background(), "ghost"); err == nil {
			t.Fatalf("expected TagGetVersion(ghost) to fail")
		}
		if err := tt.TagDelete(context.Background(), "ghost"); err == nil {
			t.Fatalf("expected TagDelete(ghost) to fail")
		}
		if err := tt.TagUpdate(context.Background(), "ghost", 2); err == nil {
			t.Fatalf("expected TagUpdate(ghost,...) to fail")
		}
		if err := tt.CheckoutTag(context.Background(), "ghost"); err == nil {
			t.Fatalf("expected CheckoutTag(ghost) to fail")
		}
		// Empty-name guards live on the Go side.
		if err := tt.TagCreate(context.Background(), "", 2); err == nil {
			t.Fatalf("expected TagCreate(\"\") to fail")
		}
		if err := tt.CheckoutTag(context.Background(), ""); err == nil {
			t.Fatalf("expected CheckoutTag(\"\") to fail")
		}

		// Pointing a tag at an unknown version must also fail.
		if err := tt.TagCreate(context.Background(), "bad", 9999); err == nil {
			t.Fatalf("expected TagCreate at unknown version to fail")
		}
	})

	t.Run("ClosedTableReturnsError", func(t *testing.T) {
		table, tt := seed(t, "tt_closed")
		if err := table.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
		if _, err := tt.ListVersions(context.Background()); err == nil {
			t.Fatalf("expected ListVersions on closed table to fail")
		}
		if err := tt.Checkout(context.Background(), 1); err == nil {
			t.Fatalf("expected Checkout on closed table to fail")
		}
		if _, err := tt.TagList(context.Background()); err == nil {
			t.Fatalf("expected TagList on closed table to fail")
		}
	})
}
