package storage

import (
	"testing"
	"time"
)

func TestBucketTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected int64
	}{
		{
			name:     "Start of hour",
			input:    time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name:     "Middle of hour",
			input:    time.Date(2025, 1, 27, 14, 30, 45, 0, time.UTC),
			expected: time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name:     "End of hour",
			input:    time.Date(2025, 1, 27, 14, 59, 59, 0, time.UTC),
			expected: time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix(),
		},
		{
			name:     "Next hour boundary",
			input:    time.Date(2025, 1, 27, 15, 0, 0, 0, time.UTC),
			expected: time.Date(2025, 1, 27, 15, 0, 0, 0, time.UTC).Unix(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bucketTimestamp(tt.input.Unix())
			if result != tt.expected {
				t.Errorf("bucketTimestamp(%v) = %d, want %d", tt.input, result, tt.expected)
				t.Errorf("  Result time: %v", time.Unix(result, 0).UTC())
				t.Errorf("  Expected:    %v", time.Unix(tt.expected, 0).UTC())
			} else {
				inputStr := tt.input.Format("15:04:05")
				outputStr := time.Unix(result, 0).UTC().Format("15:04:05")
				t.Logf("✅ %s: %s → %s", tt.name, inputStr, outputStr)
			}
		})
	}
}

func TestBucketTimestampPrivacy(t *testing.T) {
	// Messages sent within same hour should have same timestamp
	timestamps := []time.Time{
		time.Date(2025, 1, 27, 14, 5, 23, 0, time.UTC),
		time.Date(2025, 1, 27, 14, 15, 45, 0, time.UTC),
		time.Date(2025, 1, 27, 14, 32, 12, 0, time.UTC),
		time.Date(2025, 1, 27, 14, 47, 59, 0, time.UTC),
		time.Date(2025, 1, 27, 14, 58, 33, 0, time.UTC),
	}

	bucketed := make([]int64, len(timestamps))
	for i, ts := range timestamps {
		bucketed[i] = bucketTimestamp(ts.Unix())
	}

	// All should be the same
	expected := bucketed[0]
	for i, b := range bucketed {
		if b != expected {
			t.Errorf("Timestamp %d bucketed differently: %d vs %d", i, b, expected)
		}
	}

	t.Logf("✅ Privacy test passed: %d messages in same hour → single timestamp %v",
		len(timestamps), time.Unix(expected, 0).UTC())
}

func TestBucketTimestampDifferentHours(t *testing.T) {
	// Messages in different hours should have different timestamps
	hour1 := time.Date(2025, 1, 27, 14, 30, 0, 0, time.UTC)
	hour2 := time.Date(2025, 1, 27, 15, 30, 0, 0, time.UTC)
	hour3 := time.Date(2025, 1, 27, 16, 30, 0, 0, time.UTC)

	bucket1 := bucketTimestamp(hour1.Unix())
	bucket2 := bucketTimestamp(hour2.Unix())
	bucket3 := bucketTimestamp(hour3.Unix())

	if bucket1 == bucket2 || bucket2 == bucket3 || bucket1 == bucket3 {
		t.Error("Messages in different hours should have different bucketed timestamps")
	}

	t.Logf("✅ Hour separation test passed:")
	t.Logf("   14:30 → %v", time.Unix(bucket1, 0).UTC())
	t.Logf("   15:30 → %v", time.Unix(bucket2, 0).UTC())
	t.Logf("   16:30 → %v", time.Unix(bucket3, 0).UTC())
}

func TestBucketTimestampGranularity(t *testing.T) {
	// Test precision loss
	precise := time.Date(2025, 1, 27, 14, 23, 45, 123456789, time.UTC)
	bucketed := bucketTimestamp(precise.Unix())

	preciseStr := precise.Format("15:04:05.000")
	bucketedStr := time.Unix(bucketed, 0).UTC().Format("15:04:05.000")

	if preciseStr == bucketedStr {
		t.Error("Bucketing should reduce precision")
	}

	if bucketedStr != "14:00:00.000" {
		t.Errorf("Expected bucketing to 14:00:00, got %s", bucketedStr)
	}

	t.Logf("✅ Precision test: %s → %s (information loss as intended)", preciseStr, bucketedStr)
}

func TestBucketTimestampEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		checkFn  func(int64) bool
	}{
		{
			name:  "Zero timestamp",
			input: 0,
			checkFn: func(result int64) bool {
				return result == 0
			},
		},
		{
			name:  "One second before hour",
			input: time.Date(2025, 1, 27, 14, 59, 59, 0, time.UTC).Unix(),
			checkFn: func(result int64) bool {
				expected := time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix()
				return result == expected
			},
		},
		{
			name:  "Exactly on hour",
			input: time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix(),
			checkFn: func(result int64) bool {
				expected := time.Date(2025, 1, 27, 14, 0, 0, 0, time.UTC).Unix()
				return result == expected
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bucketTimestamp(tt.input)
			if !tt.checkFn(result) {
				t.Errorf("Test failed: %s", tt.name)
			} else {
				t.Logf("✅ %s passed", tt.name)
			}
		})
	}
}

func BenchmarkBucketTimestamp(b *testing.B) {
	now := time.Now().Unix()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bucketTimestamp(now)
	}
}
