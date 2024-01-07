# Apario Writer

This package is intended to become a replacement utility to the Apario Contribution
project that most of this code is taken from and built onto. That first iteration 
was necessary because it was specifically designed to ingest the JFK Files from the
official National Archives' .gov site. All official. All legit, with metadata XLSX 
analysis support. 

This has expanded into the Apario Writer project where the intended goal of this
utility will be to take resources and prepare them for consumption by the Apario
Reader application. This writer is responsible for generating what the reader 
presents to end users at the configured domain name. 

## Usage

```shell
apario-writer --help
apario-writer \
  --download-pdf-url "https://www.cia.gov/readingroom/docs/CIA-RDP96-00788R001500160012-7.pdf" \
  --database-directory "/idoread.com-data/stargate-tmp" \
  --pdf-title "STATEMENT BEFORE THE INVESTIGATIONS SUBCOMMITTEE HOUSE ARMED SERVICES" \
  --pdf-metadata "{\"Collection\":\"STARGATE\",\"Released At\":\"2004-05-17\",\"Created At\":\"2016-11-04\"}" \
  --log "/var/log/idoread.com/apario-writer.log"
```

Output: 

```log
/idoread.com-data/stargate-tmp/<checksum of url>/CIA-RDP96-00788R001500160012-7.pdf
/idoread.com-data/stargate-tmp/<checksum of url>/record.json
/idoread.com-data/stargate-tmp/<checksum of url>/extracted.json
/idoread.com-data/stargate-tmp/<checksum of url>/pages/
/idoread.com-data/stargate-tmp/<checksum of url>/pages/CIA-RDP96-00788R001500160012-7_page_1.pdf
/idoread.com-data/stargate-tmp/<checksum of url>/pages/ocr.0000001.txt
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.000001.json
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.dark.0000001.original.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.dark.0000001.large.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.dark.0000001.medium.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.dark.0000001.small.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.dark.0000001.social.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.light.0000001.original.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.light.0000001.large.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.light.0000001.medium.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.light.0000001.small.jpg
/idoread.com-data/stargate-tmp/<checksum of url>/pages/page.light.0000001.social.jpg
```

This is the default intended usage of the `apario-writer` application. 

## License

This software is released under the GPL-3 Open Source license.