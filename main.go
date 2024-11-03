package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	spotifyToken string
	playlistID   = "1ulsoy2pr1rJYMDHT4BcCm" // TODO: Playlist ID dynamisch machen
	songID       string
)

type song struct {
	id         string
	songName   string
	artistName string
	albumName  string
	songVector [14]float64
}
type Extension struct {
	Extname    string
	Extversion string
}

func main() {
	// Datenbank Anbindung
	dsn := "postgres://tsdbadmin:dgu1dhikq53f4qhx@dr1v7aqxs5.h9nt9fpvyo.tsdb.cloud.timescale.com:30467/tsdb?sslmode=require"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	spotifyToken = getSpotifyToken() // Token läuft nach einer Stunde ab

	// Playist ID vom User bekommen
	fmt.Print("Please enter a playlist from Spotify (link or ID): ")
	fmt.Scanln(&playlistID)
	if playlistID == "" {
		playlistID = "2FrqyYlVCpNzwQ6orUbG1h" // Standart Playlist ID: Wilder Mix
	}
	if strings.Contains(playlistID, "open.spotify.com") { // Ggf ID aus Link extrahieren
		playlistID = strings.Split(playlistID, "/")[len(strings.Split(playlistID, "/"))-1]
		playlistID = strings.Split(playlistID, "?")[0]
	}

	songIDs, songs, err := getSongIDsFromPlayist(playlistID) // Songs von Playlist abrufen
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get song IDs: %v\n", err)
		os.Exit(1)
	}
	songs, err = getVectorDataFromIDs(songIDs, songs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get vector data: %v\n", err)
		os.Exit(1)
	}
	createTable(playlistID, db)            // Tabelle in DB für Playlist erstellen
	insertSongsToDB(playlistID, songs, db) // Songs in DB einfügen

	// Song ID vom User bekommen
	fmt.Print("Please enter a song from Spotify (link or ID): ")
	fmt.Scanln(&songID)
	if strings.Contains(songID, "open.spotify.com") { // Ggf ID aus Link extrahieren
		songID = strings.Split(songID, "/")[len(strings.Split(songID, "/"))-1]
		songID = strings.Split(songID, "?")[0]
	}
	songs2, err := getVectorDataFromIDs(songID, []song{{id: songID}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to get vector data: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nSong Vector: %v\n", formatVector(songs2[0].songVector))
}

// Hauptlogik
func getSongIDsFromPlayist(id string) (string, []song, error) {
	songIDs := ""
	songs := []song{}
	// Request an Spotify API
	c := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/playlists/"+id+"/tracks?market=DE&limit=50&offset=0&additional_types=track", nil)
	if err != nil {
		return "", nil, fmt.Errorf("Unable to create request: %v\n", err)
	}
	req.Header = http.Header{
		"Authorization": []string{"Bearer " + spotifyToken},
	}
	resp, err := c.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("Unable to send request: %v\n", err)
	}
	body, err := GetResponseBody(resp)
	if err != nil {
		return "", nil, fmt.Errorf("Unable to read response body: %v\n", err)
	}
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(body), &jsonObj)
	if err != nil {
		return "", nil, fmt.Errorf("Unable to parse response body: %v\n", err)
	}
	for _, item := range jsonObj["items"].([]interface{}) {
		songIDs += item.(map[string]interface{})["track"].(map[string]interface{})["id"].(string) + ","
		songs = append(songs, song{
			id:         item.(map[string]interface{})["track"].(map[string]interface{})["id"].(string),
			songName:   item.(map[string]interface{})["track"].(map[string]interface{})["name"].(string),
			artistName: item.(map[string]interface{})["track"].(map[string]interface{})["artists"].([]interface{})[0].(map[string]interface{})["name"].(string),
			albumName:  item.(map[string]interface{})["track"].(map[string]interface{})["album"].(map[string]interface{})["name"].(string),
		})
	}

	if len(songIDs) > 0 {
		return songIDs[:len(songIDs)-1], songs, nil
	} else {
		return "", nil, fmt.Errorf("No songs found in playlist")
	}
}
func getVectorDataFromIDs(ids string, songs []song) ([]song, error) {
	c := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.spotify.com/v1/audio-features?ids="+ids, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create request: %v\n", err)
		os.Exit(1)
	}
	req.Header = http.Header{
		"Authorization": []string{"Bearer " + spotifyToken},
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to send request: %v\n", err)
	}
	body, err := GetResponseBody(resp)
	if err != nil {
		return nil, fmt.Errorf("Unable to read response body: %v\n", err)
	}
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(body), &jsonObj)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse response body: %v\n", err)
	}
	for i, item := range jsonObj["audio_features"].([]interface{}) {
		for j, val := range item.(map[string]interface{}) {
			if j == "acosuticness" {
				songs[i].songVector[0] = val.(float64)
			}
			if j == "danceability" {
				songs[i].songVector[1] = val.(float64)
			}
			if j == "energy" {
				songs[i].songVector[2] = val.(float64)
			}
			if j == "key" {
				songs[i].songVector[3] = val.(float64)
			}
			if j == "loudness" {
				songs[i].songVector[4] = val.(float64)
			}
			if j == "mode" {
				songs[i].songVector[5] = val.(float64)
			}
			if j == "speechiness" {
				songs[i].songVector[6] = val.(float64)
			}
			if j == "acousticness" {
				songs[i].songVector[7] = val.(float64)
			}
			if j == "instrumentalness" {
				songs[i].songVector[8] = val.(float64)
			}
			if j == "liveness" {
				songs[i].songVector[9] = val.(float64)
			}
			if j == "valence" {
				songs[i].songVector[10] = val.(float64)
			}
			if j == "tempo" {
				songs[i].songVector[11] = val.(float64)
			}
			if j == "time_signature" {
				songs[i].songVector[12] = val.(float64)
			}
			if j == "valence" {
				songs[i].songVector[13] = val.(float64)
			}
		}
	}
	return songs, nil
}

// DB
func insertSongsToDB(playlistID string, songs []song, db *gorm.DB) {
	for _, song := range songs {
		query := fmt.Sprintf("INSERT INTO %s (id, songName, artistName, albumName, songVector) VALUES ('%s', '%s', '%s', '%s', %s);", "playlist"+playlistID, song.id, song.songName, song.artistName, song.albumName, formatVector(song.songVector))
		db.Exec(query)
		fmt.Printf("\nInserted Song into DB with ID: %s, Song Name: %s, Artist Name: %s, Album Name: %s, Song Vector: %s\n", song.id, song.songName, song.artistName, song.albumName, formatVector(song.songVector))
	}
}
func createTable(playlistID string, db *gorm.DB) {
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id VARCHAR(50) PRIMARY KEY, songName VARCHAR(50), artistName VARCHAR(50), albumName VARCHAR(50), songVector VECTOR(14));", "Playlist"+playlistID)
	db.Exec(query)
}

// Spotify API
func getSpotifyToken() string {
	c := &http.Client{}
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "f3560061ef924321beca73da5afa3aab")
	data.Set("client_secret", "0c92fb786e3e4110a5bcc5861473491c")
	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", bytes.NewBufferString(data.Encode()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send request: %v\n", err)
		os.Exit(1)
	}
	body, err := GetResponseBody(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read response body: %v\n", err)
		os.Exit(1)
	}
	var jsonObj map[string]interface{}
	err = json.Unmarshal([]byte(body), &jsonObj)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to parse response body: %v\n", err)
		os.Exit(1)
	}
	return jsonObj["access_token"].(string)
}

// Sonstiges
func GetResponseBody(res *http.Response) (string, error) {
	if res == nil {
		return "", fmt.Errorf("response is nil")
	}
	defer res.Body.Close()

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
func formatVector(vector [14]float64) string {
	var strValues []string
	for _, v := range vector {
		strValues = append(strValues, fmt.Sprintf("%v", v)) // Ohne Rundung
	}
	return fmt.Sprintf("'[%s]'", strings.Join(strValues, ", "))
}
