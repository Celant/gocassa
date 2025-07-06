package gocassa

import (
	"context"
	"time"
)

// Connection represents a connection to the database, allowing keyspace management.
// Use ConnectToKeySpace to acquire a KeySpace instance without a Connection.
type Connection interface {
	CreateKeySpace(name string) error
	DropKeySpace(name string) error
	KeySpace(name string) KeySpace
	Close()
}

// KeySpace provides access to tables within a keyspace.
type KeySpace interface {
	MapTable(tableName, id string, row interface{}) MapTable
	MultimapTable(tableName, fieldToIndexBy, uniqueKey string, row interface{}) MultimapTable
	MultimapMultiKeyTable(tableName string, fieldToIndexBy, uniqueKey []string, row interface{}) MultimapMkTable
	TimeSeriesTable(tableName, timeField, uniqueKey string, bucketSize time.Duration, row interface{}) TimeSeriesTable
	MultiTimeSeriesTable(tableName, fieldToIndexByField, timeField, uniqueKey string, bucketSize time.Duration, row interface{}) MultiTimeSeriesTable
	FlexMultiTimeSeriesTable(name, timeField, idField string, indexFields []string, bucketer Bucketer, row interface{}) MultiTimeSeriesTable
	Table(tableName string, row interface{}, keys Keys) Table
	// DebugMode enables/disables debug mode depending on the value of the input boolean.
	// When DebugMode is enabled, all built CQL statements are printed to stdout.
	DebugMode(bool)
	// Name returns the keyspace name as in C*
	Name() string
	// Tables returns the names of all configured tables in the keyspace.
	Tables() ([]string, error)
	// Exists returns whether the specified table exists in the keyspace.
	Exists(string) (bool, error)
}

//
// Map recipe
//

// MapTable provides basic CRUD operations for a table.
type MapTable interface {
	Set(v interface{}) Op
	Update(id interface{}, m map[string]interface{}) Op
	Delete(id interface{}) Op
	Read(id, pointer interface{}) Op
	MultiRead(ids []interface{}, pointerToASlice interface{}) Op
	WithOptions(Options) MapTable
	TableChanger
}

//
// Multimap recipe
//

// MultimapTable allows listing rows by field equality, for example: list all sales where seller id = v.
type MultimapTable interface {
	Set(v interface{}) Op
	Update(v, id interface{}, m map[string]interface{}) Op
	Delete(v, id interface{}) Op
	DeleteAll(v interface{}) Op
	List(v, startId interface{}, limit int, pointerToASlice interface{}) Op
	Read(v, id, pointer interface{}) Op
	MultiRead(v interface{}, ids []interface{}, pointerToASlice interface{}) Op
	WithOptions(Options) MultimapTable
	TableChanger
}

// MultimapMkTable lets you list rows based on several fields equality, for example:
// list all sales where seller id = v and name = 'john'.
type MultimapMkTable interface {
	Set(v interface{}) Op
	Update(v, id map[string]interface{}, m map[string]interface{}) Op
	Delete(v, id map[string]interface{}) Op
	DeleteAll(v map[string]interface{}) Op
	List(v, startId map[string]interface{}, limit int, pointerToASlice interface{}) Op
	Read(v, id map[string]interface{}, pointer interface{}) Op
	MultiRead(v, id map[string]interface{}, pointerToASlice interface{}) Op
	WithOptions(Options) MultimapMkTable
	TableChanger
}

//
// TimeSeries recipe
//

// TimeSeriesTable lets you list rows which have a field value between two date ranges.
type TimeSeriesTable interface {
	// Set inserts or replaces a row in the table.
	// It requires the timeField and idField to be present in the struct.
	Set(v interface{}) Op
	Update(timeStamp time.Time, id interface{}, m map[string]interface{}) Op
	Delete(timeStamp time.Time, id interface{}) Op
	Read(timeStamp time.Time, id, pointer interface{}) Op
	List(start, end time.Time, pointerToASlice interface{}) Op
	WithOptions(Options) TimeSeriesTable
	TableChanger
}

//
// TimeSeries B recipe
//

// MultiTimeSeriesTable is a cross between TimeSeries and Multimap tables.
type MultiTimeSeriesTable interface {
	// Set inserts or replaces a row in the table.
	// It requires the timeField and idField to be present in the struct.
	Set(v interface{}) Op
	Update(v interface{}, timeStamp time.Time, id interface{}, m map[string]interface{}) Op
	Delete(v interface{}, timeStamp time.Time, id interface{}) Op
	Read(v interface{}, timeStamp time.Time, id, pointer interface{}) Op
	List(v interface{}, start, end time.Time, pointerToASlice interface{}) Op
	WithOptions(Options) MultiTimeSeriesTable
	TableChanger
}

//
// Raw CQL
//

// Filter represents a subset of a Table filtered by Relations. Supports reads and writes.
type Filter interface {
	// Update performs a partial update on all rows matching the filter by modifying only the specified fields in the provided map.
	// Note: If the filter matches more than one row, this operation may be inefficient and could have performance implications.
	Update(m map[string]interface{}) Op
	// Delete deletes all rows matching the filter.
	Delete() Op
	// Read reads all rows matching the filter. Make sure you pass in a pointer to a slice of structs.
	Read(pointerToASlice interface{}) Op
	// ReadOne reads a single row matching the filter. Make sure you pass in a pointer to a struct.
	ReadOne(pointer interface{}) Op
}

// Keys defines partition and clustering keys for a table.
type Keys struct {
	PartitionKeys     []string
	ClusteringColumns []string
	Compound          bool // indicates if the partitions keys are generated as compound key when no clustering columns are set
}

// Op represents one or more database operations that must be run explicitly.
type Op interface {
	// Run the operation.
	Run() error
	// RunAtomically executes the operation in a logged batch.
	// You do not need this in 95% of the use cases, use Run!
	// Using atomic batched writes (logged batches in Cassandra terminology) comes at a high performance cost!
	RunAtomically() error
	// RunWithContext runs the operation with the specified context.
	RunWithContext(context.Context) error
	// RunAtomicallyWithContext runs the operation in a logged batch with the specified context.
	RunAtomicallyWithContext(context.Context) error
	// Add another Op to this one.
	Add(...Op) Op
	// WithOptions lets you specify `Op` level `Options`.
	// The `Op` level Options and the `Table` level `Options` will be merged in a way that Op level takes precedence.
	// All queries in an `Op` will have the specified `Options`.
	// When using Add(), the existing options are preserved.
	// For example:
	//
	//    op1.WithOptions(Options{Limit:3}).Add(op2.WithOptions(Options{Limit:2})) // op1 has a limit of 3, op2 has a limit of 2
	//    op1.WithOptions(Options{Limit:3}).Add(op2).WithOptions(Options{Limit:2}) // op1 and op2 both have a limit of 2
	//
	WithOptions(Options) Op
	// Preflight performs any pre-execution validation that confirms the op considers itself "valid".
	// NOTE: Run() and RunAtomically() should call this method before execution, and abort if any errors are returned.
	Preflight() error
	// GenerateStatement generates the statement and params to perform the operation
	GenerateStatement() (string, []interface{})
	// QueryExecutor returns the QueryExecutor
	QueryExecutor() QueryExecutor
}

// TableChanger is an interface that allows you to create, recreate or drop a table.
// Danger zone! Do not use this interface unless you really know what you are doing
type TableChanger interface {
	// Create creates the table in the keySpace, but only if it does not exist already, returning an error otherwise.
	Create() error
	// CreateStatement returns you the CQL query which can be used to create the table manually in cqlsh.
	CreateStatement() (string, error)
	// CreateIfNotExist creates the table in the keySpace, but only if it does not exist already.
	CreateIfNotExist() error
	// CreateIfNotExistStatement returns you the CQL query which can be used to create the table manually in cqlsh.
	CreateIfNotExistStatement() (string, error)
	// Recreate drops the table if exists and creates it again.
	// This is useful for test purposes only.
	Recreate() error
	// Name returns the name of the table, as in C*
	Name() string
}

// Table is a non-recipe table that allows raw CQL operations. Requires knowledge of Cassandra queries.
type Table interface {
	// Set Inserts, or Replaces your row with the supplied struct. Be aware that what is not in your struct
	// will be deleted. To only overwrite some of the fields, use Query.Update.
	Set(v interface{}) Op
	// Where allows you to Filter the table by Relation(s). This is useful for reading, updating or deleting rows.
	Where(relations ...Relation) Filter // Because we provide selections
	// WithOptions allows you to specify `Options` for the table.
	WithOptions(Options) Table
	TableChanger
}

// QueryExecutor executes queries, mainly for testing or mocking. The default implementation uses github.com/gocql/gocql.
type QueryExecutor interface {
	// QueryWithOptions executes a query with the provided options and returns the results
	QueryWithOptions(opts Options, stmt string, params ...interface{}) ([]map[string]interface{}, error)
	// Query executes a query and returns the results
	Query(stmt string, params ...interface{}) ([]map[string]interface{}, error)
	// ExecuteWithOptions executes a DML query using the provided options
	ExecuteWithOptions(opts Options, stmt string, params ...interface{}) error
	// Execute executes a DML query
	Execute(stmt string, params ...interface{}) error
	// ExecuteAtomically executes multiple DML queries with a logged batch
	ExecuteAtomically(stmt []string, params [][]interface{}) error
	// ExecuteAtomicallyWithOptions executes multiple DML queries with a logged batch, using the provided options
	ExecuteAtomicallyWithOptions(opts Options, stmt []string, params [][]interface{}) error
	// Close closes the open session
	Close()
}

type Counter int
