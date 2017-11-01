# anteros

[![Build Status](https://travis-ci.org/Comcast/anteros.svg?branch=master)](https://travis-ci.org/Comcast/anteros) 
[![codecov.io](http://codecov.io/github/Comcast/anteros/coverage.svg?branch=master)](http://codecov.io/github/Comcast/anteros?branch=master)
[![Code Climate](https://codeclimate.com/github/Comcast/anteros/badges/gpa.svg)](https://codeclimate.com/github/Comcast/anteros)
[![Issue Count](https://codeclimate.com/github/Comcast/anteros/badges/issue_count.svg)](https://codeclimate.com/github/Comcast/anteros)
[![Go Report Card](https://goreportcard.com/badge/github.com/Comcast/anteros)](https://goreportcard.com/report/github.com/Comcast/anteros)
[![Apache V2 License](http://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/Comcast/anteros/blob/master/LICENSE)

The webpa upgrade redirection server.

# How to Install

## Centos 6

1. Import the public GPG key (replace `0.0.1-65` with the release you want)

```
rpm --import https://github.com/Comcast/anteros/releases/download/0.0.1-65/RPM-GPG-KEY-comcast-webpa
```

2. Install the rpm with yum (so it installs any/all dependencies for you)

```
yum install https://github.com/Comcast/anteros/releases/download/0.0.1-65/anteros-0.0.1-65.el6.x86_64.rpm
```
