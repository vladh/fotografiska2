package main

import (
	"flag"
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
const USAGE = `
fotografiska organises your photos/videos into a certain directory structure
that is easy to browse with a regular file manager.

Your photos/videos will be organised into subfolders by year and month, and
their filename will start with the date they were taken and also include a
unique hash of the file.

If the file is larger than 10MB, the hash will only be computed using the first
10MB of the file.

Here's an example. Let's say your files look like this:

	DSCF4325.JPG (taken 2021/01/01 05:23:11 +01:00)
	DSCF1234.JPG (taken 2020/08/27 11:00:00 +01:00)

You can run a command such as the following:

	fotografiska -srcDir ~/Downloads/photos -dstDir ~/Pictures

Your files will then be organised as follows:

	2020/
		02/
			2020.08.27_11.00.00+0100_b46976ab6907346a_DSCF1234.JPG
	2021/
		01/
			2020.01.01-05.23.11+0100_66f4c6bbab77a615_DSCF4325.JPG

The creation date and time will be taken from the EXIF data. When no EXIF data is
available, such as with videos, the file's modification time will be used.

Caveats:

1. Please note that if your photo/video has no EXIF data, and you've e.g. made a
copy of the file so its modification time is not the time it was taken,
fotografiska cannot correctly organise your photos into correct dates and times.

2. Always make a backup of your photos/videos before using fotografiska. It's
been reasonably tested, but it's best to be safe.
`;


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
		fmt.Printf("\tWARNING: Got DateTimeOriginal but no OffsetTimeOriginal, time will be UTC\n")
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
	hash := fmt.Sprintf("%.16x", sum)
	return hash
}


func getSortedDestination(path string, dstBaseDir string) string {
	t, err := getPhotoCreationTime(path)
	if err != nil { panic(err) }

	hash := getPhotoHash(path)

	filename := filepath.Base(path)

	dstPath := fmt.Sprintf("%s%d/%.2d/%s-%s-%s",
		dstBaseDir, t.Year(), t.Month(),
		t.Format("2006.01.02_15.04.05-0700"),
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


func validateFile(path string) bool {
	parts := strings.Split(filepath.Base(path), "-")
	if len(parts) < 3 {
		panic(fmt.Sprintf("Expected filename to split by '-' into at least 3 parts, but found %d parts: %s", len(parts), path))
	}
	hash := parts[1]
	if len(hash) != 16 {
		panic(fmt.Sprintf("Expected the following to be a length 16 hash but it wasn't: %s\nFull path was: %s", hash, path))
	}
	correctHash := getPhotoHash(path)
	return hash == correctHash
}


func sortFileIntoDestination(path string, dstBaseDir string, dryRun bool) {
	dstPath := getSortedDestination(path, dstBaseDir)
	makeDestinationDirs(dstPath)

	shouldWriteFile := false
	if _, err := os.Stat(dstPath); err == nil {
		if validateFile(dstPath) {
			fmt.Printf("\tDestination file already exists, doing nothing: %s\n", dstPath)
		} else {
			fmt.Printf("\tDestination file already exists but did not match its own hash, deleting\n")
			err := os.Remove(dstPath)
			if err != nil { panic(err) }
			shouldWriteFile = true
		}
	} else {
		shouldWriteFile = true
	}

	if shouldWriteFile {
		var bytesCopied int64 = 0
		if !dryRun {
			bytesCopied = copyFile(path, dstPath)
		}
		fmt.Printf("\t→ %s (%d bytes copied)\n", dstPath, bytesCopied)
	}
}


func main() {
	srcDirArg := flag.String("srcDir", "", "a folder containing images/videos to read (mandatory)")
	dstDirArg := flag.String("dstDir", "", "a folder to move the images/videos into (mandatory)")
	dryRunArg := flag.Bool("dryRun", false, "if true, don't actually move any files, just print out what would be done")
	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "%s [options]\n\n", os.Args[0])
		fmt.Fprintf(w, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(w, USAGE)
	}
	flag.Parse()

	if *srcDirArg == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Please specify a source directory\n\n")
		flag.Usage()
		os.Exit(1)
	}
	if *dstDirArg == "" {
		fmt.Fprintf(os.Stderr, "ERROR: Please specify a destination directory\n\n")
		flag.Usage()
		os.Exit(1)
	}

	srcDir := validateDir(*srcDirArg)
	dstBaseDir := validateDir(*dstDirArg)

	paths, err := filepath.Glob(srcDir + "**/*")
	if err != nil { panic(err) }

	for idx, path := range paths {
		fileinfo, err := os.Stat(path)
		if err != nil { panic(err) }
		if fileinfo.IsDir() {
			continue
		}
		fmt.Printf("[%.2d/%.2d] %s\n", idx + 1, len(paths), filepath.Base(path))
		sortFileIntoDestination(path, dstBaseDir, *dryRunArg)
	}
}

