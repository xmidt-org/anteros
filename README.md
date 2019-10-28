# anteros

[![No Maintenance Intended](http://unmaintained.tech/badge.svg)](http://unmaintained.tech/)
[![Go Report Card](https://goreportcard.com/badge/github.com/Comcast/anteros)](https://goreportcard.com/report/github.com/Comcast/anteros)
[![Apache V2 License](http://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/Comcast/anteros/blob/master/LICENSE)

The webpa upgrade redirection server.

## Archived

This project has been archived.  No future work will be done here.

## How to Install

### Centos 6

1. Import the public GPG key (replace `0.0.1-65` with the release you want)

```
rpm --import https://github.com/Comcast/anteros/releases/download/0.0.1-65/RPM-GPG-KEY-comcast-webpa
```

2. Install the rpm with yum (so it installs any/all dependencies for you)

```
yum install https://github.com/Comcast/anteros/releases/download/0.0.1-65/anteros-0.0.1-65.el6.x86_64.rpm
```
