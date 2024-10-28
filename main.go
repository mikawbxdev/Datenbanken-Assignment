package main

import (
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// curl --request GET \
// --url 'https://api.spotify.com/v1/playlists/1gzeoW8kBluXTsQqaK4jjX/tracks?market=DE&limit=50&offset=0&additional_types=track' \
// --header 'Authorization: Bearer f3560061ef924321beca73da5afa3aab'

func main() {
	dsn := "postgres://tsdbadmin:dgu1dhikq53f4qhx@dr1v7aqxs5.h9nt9fpvyo.tsdb.cloud.timescale.com:30467/tsdb?sslmode=require"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	type Extension struct {
		Extname    string
		Extversion string
	}

	var extensions []Extension
	db.Raw("SELECT extname, extversion from pg_extension").Scan(&extensions)
	fmt.Printf("%+v\n", extensions)
}
