package embedding

import (
	"context"
	"errors"
	"testing"
)

func TestLocalProviderEmbedsSparseVector(t *testing.T) {
	provider := NewLocalProvider()
	vector, err := provider.EmbedSparse(context.Background(), "semantic semantic roadmap")
	if err != nil {
		t.Fatalf("EmbedSparse returned error: %v", err)
	}
	if vector["semantic"] <= vector["roadmap"] {
		t.Fatalf("semantic weight = %v, roadmap weight = %v; want semantic higher", vector["semantic"], vector["roadmap"])
	}
	if got := vector.Norm(); got < 0.999 || got > 1.001 {
		t.Fatalf("normalized norm = %v, want 1", got)
	}
}

func TestLocalProviderHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewLocalProvider().EmbedSparse(ctx, "roadmap")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EmbedSparse err = %v, want context.Canceled", err)
	}
}

func TestProviderFuncAdapters(t *testing.T) {
	sparse := SparseProviderFunc(func(context.Context, string) (SparseVector, error) {
		return SparseVector{"ok": 1}, nil
	})
	vector, err := sparse.EmbedSparse(context.Background(), "ignored")
	if err != nil || vector["ok"] != 1 {
		t.Fatalf("SparseProviderFunc = %v, %v; want ok vector", vector, err)
	}

	dense := DenseProviderFunc(func(context.Context, string) (DenseVector, error) {
		return DenseVector{1, 2, 3}, nil
	})
	denseVector, err := dense.EmbedDense(context.Background(), "ignored")
	if err != nil || len(denseVector) != 3 {
		t.Fatalf("DenseProviderFunc = %v, %v; want length 3", denseVector, err)
	}
}
