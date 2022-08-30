package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dsoprea/go-exif/v3"
	"github.com/cespare/xxhash"
)


const MAX_HASHABLE_SIZE_IN_BYTES = 10485760 // 10 MB


func getExifCreationTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	defer f.Close()
	if err != nil { panic(err) }

	data, err := io.ReadAll(f)
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

	var dtStr, offsetStr string
	for _, entry := range entries {
		if entry.TagName == "DateTimeOriginal" {
			dtStr = entry.Formatted
		}
		if entry.TagName == "OffsetTimeOriginal" {
			offsetStr = entry.Formatted
		}
	}
	if dtStr == "" {
		return time.Time{}, fmt.Errorf("[%s] No DateTimeOriginal tag in EXIF data, perhaps it is named differently?", path)
	}

	if offsetStr == "" {
		fmt.Printf("[%s] WARNING: Got DateTimeOriginal but no OffsetTimeOriginal, time will be UTC", path)
		return time.Parse("2006:01:02 15:04:05", dtStr)
	} else {
		return time.Parse("2006:01:02 15:04:05-07:00", dtStr + offsetStr)
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


func getPhotoHash(path string) string {
	file, err := os.Open(path)
	defer file.Close()
	if err != nil { panic(err) }
	bytes := make([]byte, MAX_HASHABLE_SIZE_IN_BYTES)
	nBytesRead, err := file.Read(bytes)
	if err != nil || nBytesRead == 0 { panic(err) }
	sum := xxhash.Sum64(bytes)
	hash := fmt.Sprintf("%x", sum)
	return hash
}


func getSortedDestination(path string, dstBaseDir string) string {
	t, err := getPhotoCreationTime(path)
	if err != nil { panic(err) }

	hash := getPhotoHash(path)

	filename := filepath.Base(path)

	dstPath := fmt.Sprintf("%s%d/%.2d/%s-%s-%s",
		dstBaseDir, t.Year(), t.Month(),
		t.Format("2006.01.02_15.04.05"),
		hash,
		filename)

	return dstPath
}


func makeDestinationDirs(path string) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, os.ModePerm)
}


func validateDir(dirPath string) string {
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	dirinfo, err := os.Stat(dirPath)
	if err != nil { panic(err) }
	if !dirinfo.IsDir() {
		fmt.Errorf("Expected this to be a directory, but it wasn't: %s", dirPath)
	}

	return dirPath
}


func copyFile(srcPath string, dstPath string) int64 {
	srcFile, err := os.Open(srcPath)
	if err != nil { panic(err) }
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil { panic(err) }
	defer dstFile.Close()

	bytesCopied, err := io.Copy(dstFile, srcFile)
	if err != nil { panic(err) }

	return bytesCopied
}


func sortFileIntoDestination(path string, dstBaseDir string, dryRun bool) {
	dstPath := getSortedDestination(path, dstBaseDir)
	makeDestinationDirs(dstPath)

	if _, err := os.Stat(dstPath); err == nil {
		fmt.Printf("\tFile already exists, doing nothing: %s\n", dstPath)
	} else {
		var bytesCopied int64 = 0
		if !dryRun {
			bytesCopied = copyFile(path, dstPath)
		}
		fmt.Printf("\t→ %s (%d bytes copied)\n", dstPath, bytesCopied)
	}
}


func main() {
	srcDir := "/home/vladh/scratch/imgsrc"
	dstBaseDir := "/home/vladh/scratch/imgdst"
	dryRun := false

	srcDir = validateDir(srcDir)
	dstBaseDir = validateDir(dstBaseDir)

	paths, err := filepath.Glob(srcDir + "*")
	if err != nil { panic(err) }

	for idx, path := range paths {
		fmt.Printf("[%.2d/%.2d] %s\n", idx + 1, len(paths), filepath.Base(path))
		sortFileIntoDestination(path, dstBaseDir, dryRun)
	}
}

