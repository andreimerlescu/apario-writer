FROM golang:1.20.4-buster
LABEL version="v0.0.1"
LABEL description="runtime container for apario-writer dependency satisifaction"
WORKDIR /app
COPY . .
RUN rm -rf .git
RUN make install
RUN apt-get update && apt-get install -y \
    ghostscript \
    poppler-utils \
    imagemagick \
    libjpeg62-turbo-dev \
    time  \
    exiftool \
    xz-utils \
    wget \
    clamav \
    clamav-daemon \
    && rm -rf /var/lib/apt/lists/*
RUN wget https://github.com/pdfcpu/pdfcpu/releases/download/v0.6.0/pdfcpu_0.6.0_Linux_x86_64.tar.xz \
    && tar xf pdfcpu_0.4.1_Linux_x86_64.tar.xz \
    && mv pdfcpu_0.4.1_Linux_x86_64/pdfcpu /usr/local/bin \
    && rm pdfcpu_0.4.1_Linux_x86_64.tar.xz \
    && rm -rf pdfcpu_0.4.1_Linux_x86_64
RUN apt-get update && apt-get install -y \
    tesseract-ocr \
    tesseract-ocr-all \
    && rm -rf /var/lib/apt/lists/* \
RUN freshclam
RUN go build -a -race -v -o /app/apario-contribution .  \
    && chmod +x /app/apario-contribution \
    && useradd -m apario \
    && chown -R apario:apario /app
USER apario
CMD ["make", "containered"]
