// sqlite-recovery provides offline SQLite integrity and backup operations.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func main() {
	operation := flag.String("operation", "integrity", "integrity, backup, restore, or compact")
	source := flag.String("source", "", "source SQLite database path")
	target := flag.String("target", "", "target SQLite database path")
	flag.Parse()
	if *source == "" {
		fail("source path is required")
	}
	switch *operation {
	case "integrity":
		integrity(*source)
	case "backup", "restore":
		copyDatabase(*source, *target)
	case "compact":
		compact(*source)
	default:
		fail("unknown operation")
	}
}

func integrity(path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		fail("open database")
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil || result != "ok" {
		fail("integrity check failed")
	}
	fmt.Println("integrity check passed")
}

func copyDatabase(source, target string) {
	if target == "" || filepath.Clean(source) == filepath.Clean(target) {
		fail("distinct target path is required")
	}
	in, err := os.Open(source)
	if err != nil {
		fail("open source database")
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		fail("create target directory")
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail("create target database")
	}
	defer out.Close()
	if _, err := in.WriteTo(out); err != nil {
		fail("copy database")
	}
}

func compact(path string) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		fail("open database")
	}
	defer db.Close()
	if _, err := db.Exec(`VACUUM`); err != nil {
		fail("compact database")
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
