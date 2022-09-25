<!--
© 2021 Vlad-Stefan Harbuz <vlad@vladh.net>
SPDX-License-Identifier: blessing
-->

![A cartoon illustration of a camera](images/character_bouhan_camera_sm1.png)

# fotografiska

fotografiska organises your photos/videos into a certain directory structure
that is easy to browse with a regular file manager.

This is a greatly improved version of the
[original fotografiska](https://git.sr.ht/~vladh/fotografiska).

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

## Credits

Icon by [irasutoya](https://www.irasutoya.com)
