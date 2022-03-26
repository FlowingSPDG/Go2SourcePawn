module github.com/assyrianic/Go2SourcePawn

go 1.17

replace github.com/assyrianic/Go2SourcePawn/srcgo/ast_to_sp => ./srcgo/ast_to_sp

replace github.com/assyrianic/Go2SourcePawn/srcgo/ast_transform => ./srcgo/ast_transform

require (
	github.com/assyrianic/Go2SourcePawn/srcgo/ast_to_sp v0.0.0-00010101000000-000000000000 // indirect
	github.com/assyrianic/Go2SourcePawn/srcgo/ast_transform v0.0.0-00010101000000-000000000000 // indirect
)
