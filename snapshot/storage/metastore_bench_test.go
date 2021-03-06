package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

// Benchmarks returns a benchmark suite using the provided metadata store
// creation method
func Benchmarks(b *testing.B, name string, metaFn metaFactory) {
	b.Run("StatActive", makeBench(b, name, metaFn, statActiveBenchmark))
	b.Run("StatCommitted", makeBench(b, name, metaFn, statCommittedBenchmark))
	b.Run("CreateActive", makeBench(b, name, metaFn, createActiveBenchmark))
	b.Run("Remove", makeBench(b, name, metaFn, removeBenchmark))
	b.Run("Commit", makeBench(b, name, metaFn, commitBenchmark))
	b.Run("GetActive", makeBench(b, name, metaFn, getActiveBenchmark))
	b.Run("WriteTransaction", openCloseWritable(b, name, metaFn))
	b.Run("ReadTransaction", openCloseReadonly(b, name, metaFn))
}

// makeBench creates a benchmark with a writable transaction
func makeBench(b *testing.B, name string, metaFn metaFactory, fn func(context.Context, *testing.B, *MetaStore)) func(b *testing.B) {
	return func(b *testing.B) {
		ctx := context.Background()
		tmpDir, err := ioutil.TempDir("", "metastore-bench-"+name+"-")
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		ms, err := metaFn(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		ctx, t, err := ms.TransactionContext(ctx, true)
		if err != nil {
			b.Fatal(err)
		}
		defer t.Commit()

		b.ResetTimer()
		fn(ctx, b, ms)
	}
}

func openCloseWritable(b *testing.B, name string, metaFn metaFactory) func(b *testing.B) {
	return func(b *testing.B) {
		ctx := context.Background()
		tmpDir, err := ioutil.TempDir("", "metastore-bench-"+name+"-")
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		ms, err := metaFn(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, t, err := ms.TransactionContext(ctx, true)
			if err != nil {
				b.Fatal(err)
			}
			if err := t.Commit(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func openCloseReadonly(b *testing.B, name string, metaFn metaFactory) func(b *testing.B) {
	return func(b *testing.B) {
		ctx := context.Background()
		tmpDir, err := ioutil.TempDir("", "metastore-bench-"+name+"-")
		if err != nil {
			b.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		ms, err := metaFn(tmpDir)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, t, err := ms.TransactionContext(ctx, false)
			if err != nil {
				b.Fatal(err)
			}
			if err := t.Rollback(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func createActiveFromBase(ctx context.Context, ms *MetaStore, active, base string) error {
	if _, err := CreateActive(ctx, "bottom", "", false); err != nil {
		return err
	}
	if _, err := CommitActive(ctx, "bottom", base); err != nil {
		return err
	}

	_, err := CreateActive(ctx, active, base, false)
	return err
}

func statActiveBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	if err := createActiveFromBase(ctx, ms, "active", "base"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetInfo(ctx, "active")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func statCommittedBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	if err := createActiveFromBase(ctx, ms, "active", "base"); err != nil {
		b.Fatal(err)
	}
	if _, err := CommitActive(ctx, "active", "committed"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GetInfo(ctx, "committed")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func createActiveBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	for i := 0; i < b.N; i++ {
		if _, err := CreateActive(ctx, "active", "", false); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if _, _, err := Remove(ctx, "active"); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

func removeBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		if _, err := CreateActive(ctx, "active", "", false); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, _, err := Remove(ctx, "active"); err != nil {
			b.Fatal(err)
		}
	}
}

func commitBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CreateActive(ctx, "active", "", false); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		if _, err := CommitActive(ctx, "active", "committed"); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if _, _, err := Remove(ctx, "committed"); err != nil {
			b.Fatal(err)
		}
	}
}

func getActiveBenchmark(ctx context.Context, b *testing.B, ms *MetaStore) {
	var base string
	for i := 1; i <= 10; i++ {
		if _, err := CreateActive(ctx, "tmp", base, false); err != nil {
			b.Fatalf("create active failed: %+v", err)
		}
		base = fmt.Sprintf("base-%d", i)
		if _, err := CommitActive(ctx, "tmp", base); err != nil {
			b.Fatalf("commit failed: %+v", err)
		}

	}

	if _, err := CreateActive(ctx, "active", base, false); err != nil {
		b.Fatalf("create active failed: %+v", err)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := GetActive(ctx, "active"); err != nil {
			b.Fatal(err)
		}
	}
}
