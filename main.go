package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sqweek/dialog"
)

type (
	Config struct {
		Saves string `json:"saves"`
	}

	Save struct {
		Name        string
		Incarnation int
	}

	Tracked map[string]int
)

func deleteFile(s Save, c Config) error {
	incarnation := ""
	if s.Incarnation != 0 {
		incarnation = fmt.Sprintf("~%012d", s.Incarnation)
	}
	name := fmt.Sprintf("%s%s.tunic", s.Name, incarnation)

	if err := os.Remove(filepath.Join(c.Saves, name)); err != nil {
		panic(err)
	}
	fmt.Println("Deleted", name)
	return nil
}

func getIncarnation(name string) (Save, error) {
	s := strings.TrimSuffix(name, ".tunic")
	delimiter := strings.LastIndex(s, "~")
	// if no incarnation ID, start at 0
	if delimiter == -1 {
		return Save{s, 0}, nil
	}
	// attempt to convert the incarnation to an int
	incarnation, err := strconv.Atoi(s[delimiter+1:])
	if err != nil {
		return Save{s[:delimiter], -1}, err
	}

	return Save{s[:delimiter], incarnation}, nil
}

func poll(t Tracked, c Config) Tracked {
	files, err := ioutil.ReadDir(c.Saves)
	if err != nil {
		panic(err)
	}

	saves := []Save{}
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".tunic") {
			continue
		}
		save, err := getIncarnation(name)
		if err != nil {
			panic(err)
		}
		saves = append(saves, save)
	}
	// track saves we haven't seen before
	found := map[string]int{}

	for _, save := range saves {
		// parse out the save file name
		if tracked, ok := t[save.Name]; ok {
			// file is already tracked from previous polls
			// if the file doesn't match the incarnation we're protecting
			if tracked != save.Incarnation {
				// delete it
				if err := deleteFile(save, c); err != nil {
					panic(err)
				}
			}
		} else if stored, ok := found[save.Name]; ok {
			// file is new this poll but it isn't the first one
			// store which ever incarnation is higher
			if save.Incarnation > stored {
				found[save.Name] = save.Incarnation
			}
		} else {
			// file is new and has never been seen before
			found[save.Name] = save.Incarnation
		}
	}
	// loop through again to delete any new-but-superseded files
	for _, save := range saves {
		if stored, ok := found[save.Name]; ok {
			if stored != save.Incarnation {
				// delete it
				if err := deleteFile(save, c); err != nil {
					panic(err)
				}
			}
		}
	}
	// append our new frozen saves to the main list
	for name, incarnation := range found {
		fmt.Printf("Protecting %s with incarnation %d\n", name, incarnation)
		t[name] = incarnation
	}

	// look for files that got deleted to stop protecting
	for name := range t {
		found := false
		for _, save := range saves {
			if save.Name == name {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("No longer protecting %s\n", name)
			delete(t, name)
		}
	}

	return t
}

func main() {
	// load config, one way or the other
	var config Config
	if _, err := os.Stat("config.json"); err == nil {
		// config file already exists
		body, err := ioutil.ReadFile("config.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(body, &config); err != nil {
			panic(err)
		}
	} else if errors.Is(err, os.ErrNotExist) {
		// config file does not exist
		path, err := dialog.Directory().Title("TUNIC Save Directory").Browse()
		if err != nil {
			panic(err)
		}
		config = Config{
			Saves: path,
		}
		// save the config file
		payload, err := json.Marshal(config)
		if err != nil {
			panic(err)
		}
		f, err := os.Create("config.json")
		if err != nil {
			panic(err)
		}
		if _, err := f.Write(payload); err != nil {
			panic(err)
		}
		if err := f.Close(); err != nil {
			panic(err)
		}
	}

	tracked := Tracked{}
	for {
		tracked = poll(tracked, config)
		time.Sleep(1 * time.Second)
	}
}
