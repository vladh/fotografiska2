package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"io/ioutil"

	"github.com/dsoprea/go-exif/v3"
)


func getExifCreationTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil { panic(err) }

	data, err := ioutil.ReadAll(f)
	if err != nil { panic(err) }

	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil {
		if err == exif.ErrNoExif {
			return time.Time{}, err
		}
		panic(err)
	}

	entries, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil { panic(err) }

	var dt_str, offset_str string
	for _, entry := range entries {
		if entry.TagName == "DateTimeOriginal" {
			dt_str = entry.Formatted
		}
		if entry.TagName == "OffsetTimeOriginal" {
			offset_str = entry.Formatted
		}
	}
	if dt_str == "" {
		return time.Time{}, fmt.Errorf("[%s] No DateTimeOriginal tag in EXIF data, perhaps it is named differently?", path)
	}

	if offset_str == "" {
		fmt.Printf("[%s] WARNING: Got DateTimeOriginal but no OffsetTimeOriginal, time will be UTC", path)
		return time.Parse("2006:01:02 15:04:05", dt_str)
	} else {
		return time.Parse("2006:01:02 15:04:05-07:00", dt_str + offset_str)
	}
}


func getFileCtime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil { panic(err) }
	stat := fi.Sys().(*syscall.Stat_t)
	return time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
}


func getPhotoCreationTime(path string) (time.Time, error) {
	exifTime, err := getExifCreationTime(path)
	if err == nil {
		return exifTime, nil
	}
	return getFileCtime(path), nil
}


func main() {
	path := "/home/vladh/scratch/imgsrc/IMG_1520.jpg"

	t, err := getPhotoCreationTime(path)
	if err != nil { panic(err) }

	fmt.Printf("Creation time: %+v\n", t)
}

