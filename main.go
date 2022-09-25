// © 2022 Vlad-Stefan Harbuz <vlad@vladh.net>
// SPDX-License-Identifier: blessing

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/dsoprea/go-exif/v3"
	"github.com/cespare/xxhash"
)


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
			2020.08.27_11.00.00+0100-b46976ab6907346a-DSCF1234.JPG
	2021/
		01/
			2020.01.01-05.23.11+0100-66f4c6bbab77a615-DSCF4325.JPG

The creation date and time will be taken from the EXIF data. When no EXIF data is
available, such as with videos, the file's modification time will be used.

Caveats:

1. Please note that if your photo/video has no EXIF data, and you've e.g. made a
copy of the file so its modification time is not the time it was taken,
fotografiska cannot correctly organise your photos into correct dates and times.

2. Always make a backup of your photos/videos before using fotografiska. It's
been reasonably tested, but it's best to be safe.
`;


type filenameInfo struct {
	year string
	month string
	day string
	hour string
	minutes string
	seconds string
	tzoffset string
	hash string
	origFilename string
}

type timeSrc string

const (
	TIMESRC_EXIF timeSrc = "exif"
	TIMESRC_EXIF_NO_TZ timeSrc = "exif_no_tz"
	TIMESRC_FILENAME timeSrc = "filename"
	TIMESRC_CTIME timeSrc = "ctime"
)

// 2021.01.29_17.17.31_60132e3223bcaafe_IMG_E8373.JPG
var rOldFullFilename = regexp.MustCompile(`(\d\d\d\d)\.(\d\d)\.(\d\d)_(\d\d)\.(\d\d)\.(\d\d)_([0-9a-f]+)_(.*)`)
// 2008.05.17-12.52.06_IMG_3761 (1).jpeg
var rOldPlainFilename = regexp.MustCompile(`(\d\d\d\d)\.(\d\d)\.(\d\d)-(\d\d)\.(\d\d)\.(\d\d)_(.*)`)
// 2022.07.06_14.21.40+0000-c273bdc6833b42d7-DSCF0033.JPG.xmp
var rFilename = regexp.MustCompile(`(\d\d\d\d)\.(\d\d)\.(\d\d)_(\d\d)\.(\d\d)\.(\d\d)([+-]\d\d\d\d)-([0-9a-f]+)-(.*)`)

const MAX_HASHABLE_SIZE_IN_BYTES = 10485760 // 10 MB
var EXIF_EXTENSIONS = []string{
	".jpg", ".jpeg", ".jpe", ".jif", ".jfif",
	".png",
	".tif", ".tiff",
	".heic", ".heics", ".heif", ".heifs",
}
var NON_MEDIA_EXTENSIONS = []string{
	".xmp",
}


func boolAsYn(b bool) string {
	if b {
		return "y"
	}
	return "n"
}


func couldHaveExif(filename string) bool {
	filename = strings.ToLower(filename)
	for _, ext := range EXIF_EXTENSIONS {
		if strings.HasSuffix(filename, ext) {
			return true
		}
	}
	return false
}


func isMedia(filename string) bool {
	filename = strings.ToLower(filename)
	for _, ext := range NON_MEDIA_EXTENSIONS {
		if strings.HasSuffix(filename, ext) {
			return false
		}
	}
	return true
}


func getExifCreationTime(path string) (time.Time, bool, error) {
	f, err := os.Open(path)
	if err != nil { panic(err) }
	defer f.Close()

	rawExif, err := exif.SearchAndExtractExifWithReader(f)
	if err != nil {
		return time.Time{}, false, err
	}

	entries, _, err := exif.GetFlatExifData(rawExif, nil)
	if err != nil {
		return time.Time{}, false, err
	}

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
		return time.Time{}, false, fmt.Errorf("[%s] No DateTimeOriginal tag in EXIF data, perhaps it is named differently?", path)
	}

	if offsetStr == "" {
		t, err := time.Parse("2006:01:02 15:04:05", dtStr)
		return t, false, err
	} else {
		t, err := time.Parse("2006:01:02 15:04:05-07:00", dtStr + offsetStr)
		return t, true, err
	}
}


func getFileCtime(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil { panic(err) }
	stat := fi.Sys().(*syscall.Stat_t)
	ctim := time.Unix(int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec))
	mtim := time.Unix(int64(stat.Mtim.Sec), int64(stat.Mtim.Nsec))
	if (ctim.Before(mtim)) {
		return ctim
	} else {
		return mtim
	}
}


func getFilenameAdditionalInfo(path string) filenameInfo {
	groups := rOldFullFilename.FindStringSubmatch(path)
	if len(groups) > 0 {
		return filenameInfo{
			year: groups[1],
			month: groups[2],
			day: groups[3],
			hour: groups[4],
			minutes: groups[5],
			seconds: groups[6],
			hash: groups[7],
			origFilename: groups[8],
		}
	}

	groups = rOldPlainFilename.FindStringSubmatch(path)
	if len(groups) > 0 {
		return filenameInfo{
			year: groups[1],
			month: groups[2],
			day: groups[3],
			hour: groups[4],
			minutes: groups[5],
			seconds: groups[6],
			origFilename: groups[7],
		}
	}

	groups = rFilename.FindStringSubmatch(path)
	if len(groups) > 0 {
		return filenameInfo{
			year: groups[1],
			month: groups[2],
			day: groups[3],
			hour: groups[4],
			minutes: groups[5],
			seconds: groups[6],
			tzoffset: groups[7],
			hash: groups[8],
			origFilename: groups[9],
		}
	}

	return filenameInfo{}
}


func getPhotoCreationTime(path string, ai filenameInfo) (time.Time, timeSrc, error) {
	if couldHaveExif(path) {
		exifTime, haveTz, err := getExifCreationTime(path)
		if err == nil {
			if haveTz {
				return exifTime, TIMESRC_EXIF, nil
			} else {
				return exifTime, TIMESRC_EXIF_NO_TZ, nil
			}
		}
	}

	if len(ai.origFilename) > 0 {
		format := "2006.01.02_15.04.05"
		if len(ai.tzoffset) > 0 {
			format = "2006.01.02_15.04.05-0700"
		}
		datestr := fmt.Sprintf("%s.%s.%s_%s.%s.%s%s",
			ai.year, ai.month, ai.day, ai.hour, ai.minutes, ai.seconds, ai.tzoffset)
		t, err := time.Parse(format, datestr)
		return t, TIMESRC_FILENAME, err
	}

	return getFileCtime(path), TIMESRC_CTIME, nil
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


func getSortedDestination(path string, dstBaseDir string) (string, timeSrc) {
	additionalInfo := getFilenameAdditionalInfo(filepath.Base(path))
	t, tSrc, err := getPhotoCreationTime(path, additionalInfo)
	if err != nil { panic(err) }

	filename := filepath.Base(path)

	if isMedia(path) {
		hash := getPhotoHash(path)

		if len(additionalInfo.origFilename) > 0 {
			filename = additionalInfo.origFilename
		}

		dstPath := fmt.Sprintf("%s%d/%.2d/%s-%s-%s",
			dstBaseDir, t.Year(), t.Month(),
			t.Format("2006.01.02_15.04.05-0700"),
			hash,
			filename)
		return dstPath, tSrc
	} else {
		// For things that are not images or videos, we do all the usual stuff,
		// except that we leave the actual filename as-is. The reason is that
		// the filename of non-media files, such as .xmp files, often contain the
		// filename of acutal media, so for example <image-name>.xmp.
		// Therefore, the hash of the non-media file will actually be a hash
		// of the image it refers to, so it doesn't make sense for us to compute
		// a hash of the non-media file and put it in its filename.
		dstPath := fmt.Sprintf("%s%d/%.2d/%s",
			dstBaseDir, t.Year(), t.Month(), filename)
		return dstPath, TIMESRC_FILENAME
	}
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
	cmd := exec.Command("cp", "--preserve=timestamps", srcPath, dstPath)
	err := cmd.Run()
	if err != nil { panic(err) }

	filestat, err := os.Stat(dstPath)
	if err != nil { panic(err) }
	return filestat.Size()
}


func validateFile(path string) bool {
	parts := strings.Split(filepath.Base(path), "-")
	if len(parts) < 3 {
		fmt.Errorf("Expected filename to split by '-' into at least 3 parts, but found %d parts: %s", len(parts), path)
		return true
	}
	hash := parts[1]
	if len(hash) != 16 {
		fmt.Errorf("Expected the following to be a length 16 hash but it wasn't: %s\nFull path was: %s", hash, path)
		return true
	}
	correctHash := getPhotoHash(path)
	return hash == correctHash
}


func sortFileIntoDestination(path string, dstBaseDir string, dryRun bool, idx int, nFiles int) {
	dstPath, timeSrc := getSortedDestination(path, dstBaseDir)
	makeDestinationDirs(dstPath)

	var bytesCopied int64
	var doesDestExist bool
	var isDestInvalid bool

	if _, err := os.Stat(dstPath); err == nil {
		doesDestExist = true
		// We only expect media files to conform to the hash filename format.
		// There is a hashless format for non-media files, and there might be
		// other formats for other things in the future. The downside to this
		// is that we cannot validate the integrity of non-media files.
		if isMedia(dstPath) && !validateFile(dstPath) {
			isDestInvalid = true
			err := os.Remove(dstPath)
			if err != nil { panic(err) }
		}
	}

	if !dryRun && (!doesDestExist || isDestInvalid) {
		bytesCopied = copyFile(path, dstPath)
	}

	var dryRunStr string
	if dryRun {
		dryRunStr = "(dry run) "
	}

	fmt.Printf("%s[%4d/%4d] (exists? %s) (media? %s) (invalid? %s) (wrote %8db) (time from %10s) %s  ->  %s\n",
		dryRunStr, idx, nFiles,
		boolAsYn(doesDestExist), boolAsYn(isMedia(filepath.Base(path))), boolAsYn(isDestInvalid),
		bytesCopied, timeSrc, filepath.Base(path), dstPath)
}


func main() {
	var err error

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

	nFiles := 0
	err = filepath.Walk(srcDir, func(path string, fileinfo os.FileInfo, err error) error {
		if !fileinfo.IsDir() {
			nFiles += 1
		}
		return nil
	})
	if err != nil { panic(err) }

	var idx int
	err = filepath.Walk(srcDir, func(path string, fileinfo os.FileInfo, err error) error {
		if fileinfo.IsDir() {
			return nil
		}
		idx += 1
		sortFileIntoDestination(path, dstBaseDir, *dryRunArg, idx, nFiles)
		return nil
	})
	if err != nil { panic(err) }
}

