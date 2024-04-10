# use python3.9 for build
FROM python:3.9-slim-buster AS build-env

WORKDIR /app

COPY . .

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential gcc && \
    pip install --no-cache-dir -r requirements.txt

RUN pip install pyinstaller && \
    pyinstaller --onefile app.py

FROM ubuntu:latest as release-env
WORKDIR /app

# install rsync
RUN apt-get update && apt-get install -y rsync
# copy install file to container
COPY . .
COPY --from=build-env /app/dist/app /app

# install command
 CMD ["bash", "/app/install.sh", "/app"]
