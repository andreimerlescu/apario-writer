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
apario-writer \
  --download-pdf-url "https://www.cia.gov/readingroom/docs/CIA-RDP96-00788R001500160012-7.pdf" \
  --database-directory "/idoread.com-data/stargate-tmp" \
  --pdf-title "STATEMENT BEFORE THE INVESTIGATIONS SUBCOMMITTEE HOUSE ARMED SERVICES" \
  --metadata-json "{\"Collection\":\"STARGATE\",\"Released At\":\"2004-05-17\",\"Created At\":\"2016-11-04\"}" \
  --log "/var/log/idoread.com/apario-writer.log"
```

**__NOTE__**: 
1. The `--database-directory` is required on every command unless a `--config config.yaml` defines it elsewhere.
2. The `--pdf-title` is required when using `--download-pdf-url`.
3. The `--metadata-json` is always optional but must be a flattened map[string]string of data only (KEY=VALUE list).
4. The `--log` flag when omitted assumes a local directory called `logs` exists for it to write an `engine-*.log` file.
5. In addition to `--download-pdf-url` additional options can be used. Currently XLSX and CSV uploads are permitted, however the header/column titles are fixed and must be defined according to specifications. They were designed for the STARGATE files and the JFK Assassination records originally.


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

## Known Limitations

- Currently the `page.<dark|light>.#######.social.jpg` is not created in the pipeline.
- No `<basename-no-extension>.dark.pdf` file is created. Only the original PDF is downloaded and replaced with OCR.
- Extracted text may come from a PDF file whose keywords are more than 17 chars. If so, they keywords are concatenated into the extracted text.
- Not tested on Windows as there are a lot of runtime requirements. Tested on MacOS and Rocky Linux.
- Using the docker container wrapper requires knowledge of how to use Docker in a less than "hello world" manner.

## License

This software is released under the GPL-3 Open Source license.