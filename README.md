[![Go Report Card](https://goreportcard.com/badge/github.com/dakusan/gofastersql)](https://goreportcard.com/report/github.com/dakusan/gofastersql)
[![GoDoc](https://godoc.org/github.com/dakusan/gofastersql?status.svg)](https://godoc.org/github.com/dakusan/gofastersql)

![GoFasterSQL Logo](logo.jpg)

GoFasterSQL is a tool designed to enhance the efficiency and simplicity of scanning SQL rows into structures.

The flaw in the native library scanning process is its repetitive and time-consuming type determination for each row scan. It must match each field’s type with its native counterpart before converting the data string (byte array). Furthermore, the requirement to specify each individual field for scanning is tedious.

### How this works:
GoFasterSQL instead precalculates string-to-type conversions for each field, utilizing pointers to dedicated conversion functions. This approach eliminates the need for type lookups during scanning, vastly improving performance. The library offers a 2 to 2.5 times speed increase compared to native scan methods (5*+ vs sqlx), a boost that varies with the number of items in each scan. Moreover, its automatic structure determination feature is a significant time-saver during coding.

The library’s `ModelStruct` function, upon its first invocation for a list of types, determines the structure of those types through recursive reflection. These structures are then cached, allowing for swift reuse in subsequent calls to `ModelStruct`. This process needs to be executed only once, and its output is concurrency-safe.

`ModelStruct` flattens all structures and records their flattened member indexes for reading into; so row scanning is by field index, not by name. To match by name, use a `RowReaderNamed` via `StructModel.CreateReaderNamed()` (See [example #2](#Example-2) below).

`RowReader`s, created via `StructModel.CreateReader()`, are not concurrency safe and can only be used in one goroutine at a time.

### sql.Rows only (no sql.Row)
Both `ScanRow(s)` (plural and singular) functions only accept `sql.Rows` and not `sql.Row` due to the golang implementation limitations placed upon `sql.Row`. Non-plural `ScanRow` functions automatically call `Rows.Next()` and `Rows.Close()` like the native implementation.

The `SRErr()` and `*.ScanRowWErr*()` helper functions exist to help emulate sql.Row.Scan error handling functionality. See [example #3](#Example-3) below.

### Type support:
GoFasterSQL supports:
  - typedef derivatives of scalar types
  - nullable derivatives of scalar types (<code>NullType[**TYPE**]</code>)
  - **structures with nested supported types and other structures (members can also be pointers)**

The scalar types are:
  - `string`, `[]byte`, `sql.RawBytes` *(RawBytes converted to []byte for singular RowScan functions)*
  - `bool`
  - `int`, `int8`, `int16`, `int32`, `int64`
  - `uint`, `uint8`, `uint16`, `uint32`, `uint64`
  - `float32`, `float64`
  - `time.Time` *(also accepts unix timestamps ; does not currently accept typedef derivatives)*

GoFasterSQL is available under the same style of BSD license as the Go language, which can be found in the LICENSE file.

### Optimization information:
* The sole instance of reflection following a `ModelStruct` call occurs during the `ScanRow(s)` functions, where a verification ensures that the `outPointers` types align with the types specified in `ModelStruct` (the *NC versions [`DoScan(runCheck=false)`] skip this check).
* Creating a StructModel from a single structure requires much less overhead than the alternatives.
* Nested struct pointers add a very tiny bit of extra overhead over nested non-pointers.
* See [here](benchmarks/benchmarks.png) for benchmarks [[html file](benchmarks/benchmarks.html) <sup>cannot be rendered in GitHub</sup>].

# Example Usage
## Example #1
```go
package main
import (
	"database/sql"
	gf "github.com/dakusan/gofastersql"
	_ "github.com/go-sql-driver/mysql"
)

type cardCatalogIdentifier uint
type book struct {
	name          string                //Scalar type
	cardCatalogID cardCatalogIdentifier //Typedef derivative of scalar type
	student                             //Nested structure
	l             *loans                //Nested structure (pointer)
}
type student struct {
	currentBorrower   string
	currentBorrowerId int
}
type loans struct {
	libraryID *int8               //Scalar type (pointer)
	loanData  gf.NullType[[]byte] //Nullable derivative of scalar type
}

func main() {
	var db *sql.DB
	//Log into db and create books table here: CREATE TABLE books (name varchar(50), cardCatalogID int, currentBorrower varchar(50), currentBorrowerId int, libraryID tinyint, loanData varchar(50) NULL) ENGINE=MEMORY

	var b []book
	ms, err := gf.ModelStruct(book{})
	if err != nil {
		panic(err)
	}
	msr := ms.CreateReader()
	rows, _ := db.Query("SELECT * FROM books")
	for rows.Next() {
		temp := book{l: &loans{libraryID: new(int8)}}
		if err := msr.ScanRows(rows, &temp); err != nil {
			panic(err)
		}
		b = append(b, temp)
	}
	_ = rows.Close()
}
```
So:<br>
`msr.ScanRows(rows, &temp)`<br>
is equivalent to:<br>
`rows.Scan(&temp.name, &temp.cardCatalogID, &temp.currentBorrower, &temp.currentBorrowerId, temp.l.libraryID, &temp.l.loanData.Val)`<br>
and is much faster to boot!

It is also equivalent to (but a little faster than): <code>ModelStruct(<b>...</b>).CreateReader().ScanRows(rows, &temp.name, &temp.cardCatalogID, &temp.student, temp.l)</code><br>

## Example #2
Reading a single row directly into multiple variables by name
```go
//Replacement of main() from above example
var db *sql.DB
var b []book
ms, err := gf.ModelStruct(loans{}, (*student)(nil), cardCatalogIdentifier(0), "") //These are not in the same order as the below sql query
if err != nil {
	panic(err)
}
msr := ms.CreateReaderNamed()
//Param# names required due to (anonymous) top level scalars
rows, _ := db.Query("SELECT name AS Param3, cardCatalogID AS Param2, currentBorrower, currentBorrowerId, libraryID, loanData FROM books")
for rows.Next() {
	temp := book{l: &loans{libraryID: new(int8)}}
	if err := msr.ScanRows(rows, temp.l, &temp.student, &temp.cardCatalogID, &temp.name); err != nil {
		panic(err)
	}
	b = append(b, temp)
}
_ = rows.Close()
```

If you were reading just 1 row this could be simplified to:
```go
db *sql.DB
myBook := book{l: &loans{libraryID: new(int8)}}
if err := gf.ScanRowNamedWErr(
	gf.SRErr(db.Query("SELECT name AS Param3, cardCatalogID AS Param2, currentBorrower, currentBorrowerId, libraryID, loanData FROM books")),
	myBook.l, &myBook.student, &myBook.cardCatalogID, &myBook.name,
); err != nil {
	panic(err)
}
```

or most simply:
```go
db *sql.DB
myBook := book{l: &loans{libraryID: new(int8)}}
if err := gf.ScanRowNamedWErr(gf.SRErr(db.Query("SELECT * FROM books")), &myBook); err != nil {
	panic(err)
}
```

## Example #3
Reading a single row directly into multiple structs
```go
type foo struct { bar, baz int }
type moo struct { cow, calf gf.NullType[int64] }
var fooVar foo
var mooVar moo

if err := gf.ScanRowWErr(gf.SRErr(db.Query("SELECT 2, 4, 8, null")), &fooVar, &mooVar); err != nil {
	panic(err)
}

//This is equivalent to the above statement but takes a little less processing
if err := gf.ScanRowWErr(gf.SRErr(db.Query("SELECT 2, 4, 8, null")), &struct {*foo; *moo}{&fooVar, &mooVar}); err != nil {
	panic(err)
}

```
Result:
```go
	fooVar = {2, 4}
	mooVar = {8, NULL}
```

> [!warning]
> If you are scanning a lot of rows it is recommended to use a `RowReader` instead of `gofastersql.ScanRow` as it bypasses a mutex read lock and a few allocations.
>
> In some cases `gofastersql.ScanRow` may even be slower than the native `sql.Row.Scan()` method. What speeds this library up so much is the preprocessing done before the ScanRow(s) functions are called and a lot of that is lost in `gofastersql.ScanRow` and especially in `gofastersql.ScanRowMulti`.

# Installation
GoFasterSQL is available using the standard go get command.

Install by running:

go get github.com/dakusan/gofastersql

# TODO:
The column name matching algorithm for `CreateReaderNamed` could be greatly improved. If there turns out to be a large need for this by users, I’ll rework it. Right now it works by matching each column in order via the following formula:
1) Search through each field in order:
    1) Skip any field that has already been matched
    2) Find any field whose full path matches the column name exactly and use it as the match
    3) Find exactly 1 field whose base name matches the column name exactly. If more than 1 is found, an error is returned.
