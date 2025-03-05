// Package mongo provides MongoDB database connectivity and repositories.
package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"norelock.dev/listenify/backend/internal/config"
	"norelock.dev/listenify/backend/internal/utils"
)

// Client wraps the MongoDB client with app-specific functionality
type Client struct {
	client   *mongo.Client
	database string
	logger   *utils.Logger
}

// NewClient creates a new MongoDB client
func NewClient(cfg *config.Config, logger *utils.Logger) (*Client, error) {
	// If no logger is provided, use the global logger
	if logger == nil {
		logger = utils.GetLogger()
	}

	// Create MongoDB client options
	clientOptions := options.Client().
		ApplyURI(cfg.Database.MongoDB.URI).
		SetMaxPoolSize(cfg.Database.MongoDB.MaxPoolSize).
		SetMinPoolSize(cfg.Database.MongoDB.MinPoolSize).
		SetMaxConnIdleTime(cfg.Database.MongoDB.MaxIdleTime)

	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Database.MongoDB.Timeout)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		logger.Error("Failed to connect to MongoDB", err)
		return nil, err
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		logger.Error("Failed to ping MongoDB", err)
		return nil, err
	}

	logger.Info("Connected to MongoDB", "uri", cfg.Database.MongoDB.URI, "database", cfg.Database.MongoDB.Database)

	return &Client{
		client:   client,
		database: cfg.Database.MongoDB.Database,
		logger:   logger,
	}, nil
}

// Database returns the MongoDB database
func (c *Client) Database() *mongo.Database {
	return c.client.Database(c.database)
}

// Collection returns a MongoDB collection
func (c *Client) Collection(name string) *mongo.Collection {
	return c.Database().Collection(name)
}

// Client returns the underlying MongoDB client
func (c *Client) Client() *mongo.Client {
	return c.client
}

// Disconnect closes the MongoDB connection
func (c *Client) Disconnect(ctx context.Context) error {
	err := c.client.Disconnect(ctx)
	if err != nil {
		c.logger.Error("Failed to disconnect from MongoDB", err)
		return err
	}
	c.logger.Info("Disconnected from MongoDB")
	return nil
}

// EnsureIndexes ensures that all required indexes are created
func (c *Client) EnsureIndexes(ctx context.Context) error {
	c.logger.Info("Ensuring MongoDB indexes")

	// Create indexes for users collection
	if err := ensureUserIndexes(ctx, c); err != nil {
		return err
	}

	// Create indexes for rooms collection
	if err := ensureRoomIndexes(ctx, c); err != nil {
		return err
	}

	// Create indexes for media collection
	if err := ensureMediaIndexes(ctx, c); err != nil {
		return err
	}

	// Create indexes for playlists collection
	if err := ensurePlaylistIndexes(ctx, c); err != nil {
		return err
	}

	// Create indexes for chat collection
	if err := ensureChatIndexes(ctx, c); err != nil {
		return err
	}

	// Create indexes for history collection
	if err := ensureHistoryIndexes(ctx, c); err != nil {
		return err
	}

	c.logger.Info("MongoDB indexes created successfully")
	return nil
}

// WithTransaction executes a function within a MongoDB transaction
func (c *Client) WithTransaction(ctx context.Context, fn func(sessCtx context.Context) (any, error)) (any, error) {
	// Start a session
	session, err := c.client.StartSession()
	if err != nil {
		c.logger.Error("Failed to start MongoDB session", err)
		return nil, err
	}
	defer session.EndSession(ctx)

	// Execute the transaction
	result, err := session.WithTransaction(ctx, fn)
	if err != nil {
		c.logger.Error("MongoDB transaction failed", err)
		return nil, err
	}

	return result, nil
}

// WithContext creates a context with timeout
func (c *Client) WithContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

// DatabaseName returns the name of the database
func (c *Client) DatabaseName() string {
	return c.database
}

// Logger returns the logger used by the client
func (c *Client) Logger() *utils.Logger {
	return c.logger
}
