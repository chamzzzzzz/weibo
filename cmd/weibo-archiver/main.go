package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chamzzzzzz/weibo"
)

type Config struct {
	Cookie      string
	Userids     []string
	Destination string
}

type Stat struct {
	Userid   string
	Mblogs   []*weibo.Mblog
	Fetched  int
	Skipped  int
	Failed   int
	Archived int
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatal("Failed to load config: ", err)
	}
	if config.Cookie == "" {
		config.Cookie = weibo.Cookie
	}
	if config.Destination == "" {
		config.Destination = "data"
	}

	err = os.MkdirAll(config.Destination, 0755)
	if err != nil {
		if !os.IsExist(err) {
			log.Fatal("Failed to create destination directory: ", err)
		}
	}

	client := &weibo.Client{
		Cookie: config.Cookie,
	}
	var stats []*Stat
	for _, userid := range config.Userids {
		pageCount := 1
		dir := filepath.Join(config.Destination, userid)
		fi, err := os.Stat(dir)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Failed to stat user [%s] directory [%s]. error:'%v'", userid, dir, err)
				continue
			}
			pageCount = 2
		}
		if fi != nil && !fi.IsDir() {
			log.Printf("User [%s] directory [%s] is not a directory.", userid, dir)
			continue
		}

		mblogs, err := get(client, userid, pageCount)
		if err != nil {
			log.Printf("Failed to get mblogs for user [%s]. error:'%v'", userid, err)
			continue
		}

		err = os.MkdirAll(dir, 0755)
		if err != nil {
			if !os.IsExist(err) {
				log.Printf("Failed to create user [%s] directory [%s]. error:'%v'", userid, dir, err)
				continue
			}
		}

		stat := &Stat{
			Userid:  userid,
			Fetched: len(mblogs),
		}
		stats = append(stats, stat)

		groupByMonth := make(map[string][]*weibo.Mblog)
		for _, mblog := range mblogs {
			d, err := time.ParseInLocation(time.RubyDate, mblog.CreatedAt, time.Local)
			if err != nil {
				stat.Failed++
				log.Printf("Failed to parse mblog created_at [%s]. error:'%v'", mblog.CreatedAt, err)
				continue
			}
			date := d.Format("2006-01")
			groupByMonth[date] = append(groupByMonth[date], mblog)
		}

		for date, mblogs := range groupByMonth {
			file := filepath.Join(dir, date+".txt")
			_mblogs, err := load(file)
			if err != nil {
				stat.Failed += len(mblogs)
				log.Printf("Failed to load mblogs from file [%s]. error:'%v'", file, err)
				continue
			}

			skipped, archived := 0, 0
			for _, mblog := range mblogs {
				if has(_mblogs, mblog) {
					skipped++
					continue
				}
				_mblogs = append(_mblogs, mblog)
				archived++
			}
			if archived > 0 {
				err = save(file, _mblogs)
				if err != nil {
					stat.Failed += len(mblogs)
					log.Printf("Failed to save mblogs to file [%s]. error:'%v'", file, err)
					continue
				}
			}
			stat.Skipped += skipped
			stat.Archived += archived
		}

	}
	for _, stat := range stats {
		log.Printf("User [%s] stats: fetched:%d archived:%d skipped:%d failed:%d", stat.Userid, stat.Fetched, stat.Archived, stat.Skipped, stat.Failed)
	}
}

func loadConfig() (*Config, error) {
	b, err := os.ReadFile("config.json")
	if err != nil {
		return nil, err
	}
	config := &Config{}
	err = json.Unmarshal(b, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func get(client *weibo.Client, userid string, pageCount int) ([]*weibo.Mblog, error) {
	var mblogs []*weibo.Mblog
	for page := 1; page <= pageCount; page++ {
		_mblogs, err := client.GetMblogs(userid, page, true)
		if err != nil {
			log.Printf("Failed to get mblogs for user [%s] page [%d]. error:'%v'", userid, page, err)
			return nil, err
		}
		mblogs = append(mblogs, _mblogs...)
		log.Printf("Fetched [%d] mblogs for user [%s] page [%d].", len(_mblogs), userid, page)
	}
	return mblogs, nil
}

func load(file string) ([]*weibo.Mblog, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var mblogs []*weibo.Mblog
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		mblog := &weibo.Mblog{}
		fields := strings.SplitN(line, "|", 4)
		if len(fields) != 4 {
			continue
		}
		mblog.CreatedAt = fields[0]
		mblog.ID, _ = strconv.ParseInt(fields[1], 10, 64)
		mblog.MblogID = fields[2]
		mblog.TextRaw = fields[3]
		mblogs = append(mblogs, mblog)
	}
	return mblogs, nil
}

func save(file string, mblogs []*weibo.Mblog) error {
	sort.Slice(mblogs, func(i, j int) bool {
		di, ei := time.ParseInLocation(time.RubyDate, mblogs[i].CreatedAt, time.Local)
		dj, ej := time.ParseInLocation(time.RubyDate, mblogs[j].CreatedAt, time.Local)
		if ei != nil || ej != nil {
			return mblogs[i].ID < mblogs[j].ID
		}
		return di.Before(dj)
	})
	var lines []string
	for _, mblog := range mblogs {
		lines = append(lines, strings.Join([]string{mblog.CreatedAt, strconv.FormatInt(mblog.ID, 10), mblog.MblogID, strip(mblog.TheText())}, "|"))
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0644)
}

func has(mblogs []*weibo.Mblog, mblog *weibo.Mblog) bool {
	for _, _mblog := range mblogs {
		if strip(_mblog.TheText()) == strip(mblog.TheText()) {
			return true
		}
	}
	return false
}

func strip(text string) string {
	return strings.ReplaceAll(text, "\n", "\\n")
}
