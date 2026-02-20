package decoder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

const masterFileURL = "https://raw.githubusercontent.com/WatWowMap/Masterfile-Generator/master/master-latest-basics.json"

var masterFileCachePath = "cache/master-latest-basics.json"
var masterFileHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

var (
	errMasterFileFetch      = errors.New("can't fetch remote MasterFile")
	errMasterFileOpen       = errors.New("can't open MasterFile")
	errMasterFileUnmarshall = errors.New("can't unmarshall MasterFile")
	errMasterFileMarshall   = errors.New("can't marshall MasterFile")
	errMasterFileSave       = errors.New("can't save MasterFile")
)

var boostedWeatherLookup = [...]uint8{0, 8, 16, 32, 16, 2, 8, 4, 128, 64, 2, 4, 2, 4, 32, 64, 32, 128, 16}

type MasterFileData struct {
	Initialized bool                      `json:"-"`
	Pokemon     map[int]MasterFilePokemon `json:"pokemon"`
	Costumes    map[int]bool              `json:"costumes"`
}

type MasterFilePokemon struct {
	Name  string                 `json:"name"`
	Types []int                  `json:"types"`
	Forms map[int]MasterFileForm `json:"forms"`
}

type MasterFileForm struct {
	Types []int `json:"types"`
}

type rawMasterFile struct {
	Pokemon  map[string]rawMasterFilePokemon `json:"pokemon"`
	Costumes map[string]json.RawMessage      `json:"costumes"`
}

type rawMasterFilePokemon struct {
	Name  string                              `json:"name,omitempty"`
	Types []int                               `json:"types"`
	Forms map[string]rawMasterFilePokemonForm `json:"forms"`
}

type rawMasterFilePokemonForm struct {
	Types []int `json:"types"`
}

type masterFileStore struct {
	mu sync.RWMutex

	raw  []byte
	data MasterFileData

	watcherChan chan bool
}

var masterFiles = &masterFileStore{}

func EnsureMasterFileData() error {
	return masterFiles.Ensure()
}

func (s *masterFileStore) Ensure() error {
	if err := s.Fetch(); err != nil {
		log.Warnf("MasterFile fetch failed: %v", err)
		if err2 := s.Load(""); err2 != nil {
			log.Warnf("Loading MasterFile from cache failed: %v", err2)
			if err3 := s.Load("pogo/master-latest-basics.json"); err3 != nil {
				return fmt.Errorf("masterfile unavailable (fetch: %w, cache: %v, fallback: %v)", err, err2, err3)
			}
			log.Warnf("Loaded MasterFile from bundled fallback")
		} else {
			log.Warnf("Loaded MasterFile from cache")
		}
	} else {
		log.Infof("MasterFile fetched successfully")
		if err := s.Save(); err != nil {
			log.Warnf("Storing MasterFile cache under %s has failed: %v", masterFileCachePath, err)
		}
	}
	return nil
}

// FetchMasterFileData downloads and loads the remote masterfile.
func FetchMasterFileData() error {
	return masterFiles.Fetch()
}

// Fetch downloads and loads the remote masterfile.
func (s *masterFileStore) Fetch() error {
	data, err := downloadMasterFile()
	if err != nil {
		return err
	}
	return s.loadMasterFileBytes(data)
}

// LoadMasterFileData loads the masterfile from disk.
func LoadMasterFileData(filePath string) error {
	return masterFiles.Load(filePath)
}

// Load loads the masterfile from disk.
func (s *masterFileStore) Load(filePath string) error {
	if filePath == "" {
		filePath = masterFileCachePath
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return errMasterFileOpen
	}
	return s.loadMasterFileBytes(data)
}

// SaveMasterFileData writes the raw masterfile to cache.
func SaveMasterFileData() error {
	return masterFiles.Save()
}

// Save writes the raw masterfile to cache.
func (s *masterFileStore) Save() error {
	s.mu.RLock()
	if len(s.raw) == 0 {
		s.mu.RUnlock()
		return errMasterFileMarshall
	}
	raw := make([]byte, len(s.raw))
	copy(raw, s.raw)
	s.mu.RUnlock()

	if err := os.WriteFile(masterFileCachePath, raw, 0644); err != nil {
		return errMasterFileSave
	}
	return nil
}

func WatchMasterFileData() error {
	return masterFiles.Watch()
}

func (s *masterFileStore) Watch() error {
	s.mu.Lock()
	if s.watcherChan != nil {
		s.mu.Unlock()
		return errors.New("MasterFile watcher is already running")
	}
	watcherChan := make(chan bool)
	s.watcherChan = watcherChan
	s.mu.Unlock()
	log.Infof("MasterFile watcher started")
	interval := 60 * time.Minute

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-watcherChan:
				log.Infof("MasterFile watcher stopped")
				return
			case <-ticker.C:
				log.Infof("Checking remote MasterFile")
				data, err := downloadMasterFile()
				if err != nil {
					log.Infof("Remote MasterFile fetch failed: %v", err)
					continue
				}
				s.mu.RLock()
				same := bytes.Equal(s.raw, data)
				s.mu.RUnlock()
				if same {
					continue
				}
				if err := s.loadMasterFileBytes(data); err != nil {
					log.Warnf("Unable to parse new MasterFile: %v", err)
					continue
				}
				if err := s.Save(); err != nil {
					log.Warnf("Storing MasterFile cache under %s has failed: %v", masterFileCachePath, err)
				} else {
					log.Infof("MasterFile cache saved to %s", masterFileCachePath)
					reloadOhbemFromMasterFile()
				}
			}
		}
	}()
	return nil
}

func (s *masterFileStore) Initialized() bool {
	s.mu.RLock()
	initialized := s.data.Initialized
	s.mu.RUnlock()
	return initialized
}

func (s *masterFileStore) Snapshot() MasterFileData {
	s.mu.RLock()
	data := s.data
	s.mu.RUnlock()
	return data
}

func (s *masterFileStore) Pokemon(pokemonID int16) (MasterFilePokemon, bool) {
	data := s.Snapshot()
	pokemon, ok := data.Pokemon[int(pokemonID)]
	if !ok {
		log.Warnf("MasterFile: Unknown PokemonId %d", pokemonID)
		return MasterFilePokemon{}, false
	}
	return pokemon, true
}

func (s *masterFileStore) BoostedWeathers(pokemonID, form int16) (result uint8) {
	return boostedWeathersFromData(s.Snapshot(), pokemonID, form)
}

func downloadMasterFile() ([]byte, error) {
	req, err := http.NewRequest("GET", masterFileURL, nil)
	if err != nil {
		return nil, errMasterFileFetch
	}
	req.Header.Set("User-Agent", "Golbat")

	resp, err := masterFileHTTPClient.Do(req)
	if err != nil {
		return nil, errMasterFileFetch
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errMasterFileFetch
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errMasterFileFetch
	}
	return body, nil
}

func (s *masterFileStore) loadMasterFileBytes(data []byte) error {
	var raw rawMasterFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return errMasterFileUnmarshall
	}

	parsed := MasterFileData{
		Pokemon:  make(map[int]MasterFilePokemon, len(raw.Pokemon)),
		Costumes: make(map[int]bool, len(raw.Costumes)),
	}

	for pid, pokemon := range raw.Pokemon {
		intPid, err := strconv.Atoi(pid)
		if err != nil {
			continue
		}
		forms := make(map[int]MasterFileForm, len(pokemon.Forms))
		for fid, form := range pokemon.Forms {
			intFid, err := strconv.Atoi(fid)
			if err != nil {
				continue
			}
			forms[intFid] = MasterFileForm{
				Types: append([]int(nil), form.Types...),
			}
		}
		parsed.Pokemon[intPid] = MasterFilePokemon{
			Name:  pokemon.Name,
			Types: append([]int(nil), pokemon.Types...),
			Forms: forms,
		}
	}

	for cid := range raw.Costumes {
		if intCid, err := strconv.Atoi(cid); err == nil {
			parsed.Costumes[intCid] = true
		}
	}

	parsed.Initialized = true

	s.mu.Lock()
	s.data = parsed

	s.raw = make([]byte, len(data))
	copy(s.raw, data)
	s.mu.Unlock()
	return nil
}

func boostedWeathersFromData(data MasterFileData, pokemonID, form int16) (result uint8) {
	pokemon, ok := data.Pokemon[int(pokemonID)]
	if !ok {
		log.Warnf("Unknown PokemonId %d", pokemonID)
		return
	}
	if form > 0 {
		formData, ok := pokemon.Forms[int(form)]
		if !ok {
			log.Warnf("Unknown Form %d for PokemonId %d", form, pokemonID)
		} else if formData.Types != nil {
			for _, t := range formData.Types {
				result |= boostedWeatherLookup[t]
			}
			return
		}
	}
	for _, t := range pokemon.Types {
		result |= boostedWeatherLookup[t]
	}
	return
}
