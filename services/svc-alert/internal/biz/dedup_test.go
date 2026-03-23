package biz

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint(t *testing.T) {
	t.Run("same inputs produce same fingerprint", func(t *testing.T) {
		labels := map[string]string{"host": "web-01", "service": "auth"}
		fp1 := Fingerprint("rule-1", labels)
		fp2 := Fingerprint("rule-1", labels)
		assert.Equal(t, fp1, fp2)
	})

	t.Run("different rule IDs produce different fingerprints", func(t *testing.T) {
		labels := map[string]string{"host": "web-01"}
		fp1 := Fingerprint("rule-1", labels)
		fp2 := Fingerprint("rule-2", labels)
		assert.NotEqual(t, fp1, fp2)
	})

	t.Run("different labels produce different fingerprints", func(t *testing.T) {
		fp1 := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		fp2 := Fingerprint("rule-1", map[string]string{"host": "web-02"})
		assert.NotEqual(t, fp1, fp2)
	})

	t.Run("label order does not affect fingerprint", func(t *testing.T) {
		labels1 := map[string]string{"a": "1", "b": "2", "c": "3"}
		labels2 := map[string]string{"c": "3", "a": "1", "b": "2"}
		fp1 := Fingerprint("rule-1", labels1)
		fp2 := Fingerprint("rule-1", labels2)
		assert.Equal(t, fp1, fp2)
	})

	t.Run("fingerprint has sha256 prefix", func(t *testing.T) {
		fp := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		assert.Contains(t, fp, "sha256:")
	})

	t.Run("empty labels produce stable fingerprint", func(t *testing.T) {
		fp1 := Fingerprint("rule-1", map[string]string{})
		fp2 := Fingerprint("rule-1", map[string]string{})
		assert.Equal(t, fp1, fp2)
	})

	t.Run("nil labels produce stable fingerprint", func(t *testing.T) {
		fp1 := Fingerprint("rule-1", nil)
		fp2 := Fingerprint("rule-1", nil)
		assert.Equal(t, fp1, fp2)
	})
}

func TestDeduplicate(t *testing.T) {
	t.Run("first occurrence is not duplicate", func(t *testing.T) {
		d := NewDeduplicator(1 * time.Minute)
		fp := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		assert.False(t, d.IsDuplicate(fp))
	})

	t.Run("same fingerprint within window is duplicate", func(t *testing.T) {
		d := NewDeduplicator(1 * time.Minute)
		fp := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		d.Record(fp)
		assert.True(t, d.IsDuplicate(fp))
	})

	t.Run("different fingerprints are not duplicates", func(t *testing.T) {
		d := NewDeduplicator(1 * time.Minute)
		fp1 := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		fp2 := Fingerprint("rule-2", map[string]string{"host": "web-01"})
		d.Record(fp1)
		assert.False(t, d.IsDuplicate(fp2))
	})

	t.Run("expired fingerprint is not duplicate", func(t *testing.T) {
		d := NewDeduplicator(50 * time.Millisecond)
		fp := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		d.Record(fp)

		// Wait for expiration
		time.Sleep(100 * time.Millisecond)
		assert.False(t, d.IsDuplicate(fp))
	})

	t.Run("re-recording refreshes window", func(t *testing.T) {
		d := NewDeduplicator(1 * time.Minute)
		fp := Fingerprint("rule-1", map[string]string{"host": "web-01"})
		d.Record(fp)
		require.True(t, d.IsDuplicate(fp))

		// Record again
		d.Record(fp)
		assert.True(t, d.IsDuplicate(fp))
	})
}
