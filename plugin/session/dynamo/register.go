package dynamo

import "github.com/shibukawa/popcornwave/database/dynamo"

// Importing this package puts the session table into the desired schema, so
// pw migrate creates it in development and startup verification checks it
// everywhere. A project that does not import the package contributes no table.
func init() {
	dynamo.RegisterTable(DeclaredTable, Table)
}
