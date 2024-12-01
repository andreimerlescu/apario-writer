#!/bin/bash

sudo wget https://github.com/pdfcpu/pdfcpu/releases/download/v0.9.1/pdfcpu_0.9.1_Linux_x86_64.tar.xz
sudo tar xf pdfcpu_0.9.1_Linux_x86_64.tar.xz
sudo mv pdfcpu_0.9.1_Linux_x86_64/pdfcpu /usr/local/bin
sudo rm -f pdfcpu_0.9.1_Linux_x86_64.tar.xz
sudo rm -rf pdfcpu_0.9.1_Linux_x86_64
sudo yum install ghostscript
sudo yum install pdftotext
sudo yum install ImageMagick
sudo yum install tesseract
sudo yum install libjpeg-turbo-devel
sudo yum -y install clamav-server clamav-data clamav-update clamav-filesystem clamav clamav-scanner-systemd clamav-devel clamav-lib clamav-server-systemd
sudo setsebool -P antivirus_can_scan_system 1
sudo setsebool -P clamd_use_jit 1
sudo getsebool -a | grep antivirus
sudo sed -i -e "s/^Example/#Example/" /etc/clamd.d/scan.conf
sudo sed -i -e "s/^##LocalSocket \/var\/run\/clamd.scan\/clamd.sock/LocalSocket \/var\/run\/clamd.scan\/clamd.sock/" /etc/clamd.d/scan.conf
sudo sed -i -e "s/^Example/#Example/" /etc/freshclam.conf
sudo freshclam
sudo systemctl start clamd@scan
sudo systemctl enable clamd@scan
make build
