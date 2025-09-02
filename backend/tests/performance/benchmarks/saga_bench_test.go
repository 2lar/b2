//go:build bench
// +build bench

// Package benchmarks provides performance benchmarks for critical paths
package benchmarks

import (
	"context"
	"testing"

	"brain2-backend/internal/core/application/cqrs"
	"brain2-backend/internal/core/application/ports"
	"brain2-backend/internal/core/application/sagas"
	"brain2-backend/tests/fixtures/builders"
)

// BenchmarkCreateNodeSaga benchmarks the node creation saga
func BenchmarkCreateNodeSaga(b *testing.B) {
	ctx := context.Background()
	
	// Setup nil dependencies for benchmark (would be mocks in real test)
	var commandBus cqrs.CommandBus
	var nodeRepo ports.NodeRepository
	var edgeRepo ports.EdgeRepository
	var keywordService ports.KeywordExtractor
	var connectionSvc ports.ConnectionAnalyzer
	var searchService ports.SearchService
	var logger ports.Logger
	var metrics ports.Metrics
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		saga := sagas.NewCreateNodeSaga(
			commandBus,
			nodeRepo,
			edgeRepo,
			keywordService,
			connectionSvc,
			searchService,
			logger,
			metrics,
			"user-123",
			"Benchmark content",
			"Benchmark title",
			[]string{"tag1", "tag2"},
			[]string{"cat1"},
		)
		
		// Execute saga steps (without actual I/O)
		_ = saga.Execute(ctx)
	}
}

// BenchmarkNodeBuilder benchmarks the node builder
func BenchmarkNodeBuilder(b *testing.B) {
	b.Run("SimpleNode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Test content").
				Build()
		}
	})
	
	b.Run("RichNode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = builders.NewNodeBuilder().
				WithUserID("user-123").
				WithTitle("Rich Node").
				WithContent("Complex content with lots of text").
				WithTags("tag1", "tag2", "tag3", "tag4", "tag5").
				WithKeywords("keyword1", "keyword2", "keyword3").
				WithCategories("cat1", "cat2").
				WithMetadata("key1", "value1").
				WithMetadata("key2", "value2").
				Build()
		}
	})
	
	b.Run("WithEvents", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Test content").
				BuildWithEvents()
		}
	})
}

// BenchmarkEventBuilder benchmarks the event builder
func BenchmarkEventBuilder(b *testing.B) {
	b.Run("NodeCreatedEvent", func(b *testing.B) {
		builder := builders.NewEventBuilder()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = builder.BuildNodeCreated(
				"Content",
				"Title",
				[]string{"tag1", "tag2"},
			)
		}
	})
	
	b.Run("NodeUpdatedEvent", func(b *testing.B) {
		builder := builders.NewEventBuilder()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = builder.BuildNodeUpdated(
				"Updated content",
				"Updated title",
				[]string{"tag1", "tag2"},
			)
		}
	})
	
	b.Run("EventSequence", func(b *testing.B) {
		presets := builders.NewEventBuilderPresets()
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			_ = presets.EventSequence("node-123", "user-123")
		}
	})
}

// BenchmarkBulkOperations benchmarks bulk operations
func BenchmarkBulkOperations(b *testing.B) {
	nodeIDs := make([]string, 100)
	for i := range nodeIDs {
		nodeIDs[i] = "node-" + string(rune(i))
	}
	
	b.Run("BulkDelete", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var commandBus cqrs.CommandBus
			var nodeRepo ports.NodeRepository
			var eventBus ports.EventBus
			var logger ports.Logger
			var metrics ports.Metrics
			
			saga := sagas.NewBulkOperationSaga(
				commandBus,
				nodeRepo,
				eventBus,
				logger,
				metrics,
				"user-123",
				sagas.BulkOperationDelete,
				nodeIDs,
				nil, // targetData
			)
			
			// Just create the saga, don't execute (no mocks)
			_ = saga
		}
	})
	
	b.Run("BulkArchive", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var commandBus cqrs.CommandBus
			var nodeRepo ports.NodeRepository
			var eventBus ports.EventBus
			var logger ports.Logger
			var metrics ports.Metrics
			
			saga := sagas.NewBulkOperationSaga(
				commandBus,
				nodeRepo,
				eventBus,
				logger,
				metrics,
				"user-123",
				sagas.BulkOperationArchive,
				nodeIDs,
				nil, // targetData
			)
			
			_ = saga
		}
	})
}

// BenchmarkConcurrentNodeCreation benchmarks concurrent node creation
func BenchmarkConcurrentNodeCreation(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = builders.NewNodeBuilder().
				WithUserID("user-123").
				WithContent("Concurrent test").
				Build()
		}
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("NodeAggregate", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = builders.NewNodeBuilder().Build()
		}
	})
	
	b.Run("EventList", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			events := make([]interface{}, 0, 10)
			for j := 0; j < 10; j++ {
				events = append(events, builders.NewEventBuilder().BuildNodeCreated("content", "title", nil))
			}
		}
	})
}