# Masquerade 🎭

[![Tests](https://github.com/joeig/masquerade/workflows/Tests/badge.svg)](https://github.com/joeig/masquerade/actions)
[![Test coverage](https://img.shields.io/badge/coverage-75%25-success)](https://github.com/joeig/masquerade/tree/master/.github/testcoverage.yml)
[![Go Report Card](https://goreportcard.com/badge/go.eigsys.de/masquerade)](https://goreportcard.com/report/go.eigsys.de/masquerade)

Masquerade helps you to host your Go modules behind your own domain.
It verifies whether the desired module exists in your GitHub account and directs `go get` (and other tools) to the right repository URL.

## Setup

    go install go.eigsys.de/masquerade/cmd/masquerade@latest

## Usage

### Quick start

    $ masquerade -packageHost "go.eigsys.de" -githubOwner "joeig"

### Print the full usage

    $ masquerade -help

## Notes

* For performance reasons, Masquerade caches all GitHub responses for one hour in memory.
  You can clear the cache by restarting the application.
* Furthermore, a `Cache-Control` header is set with each response, which instructs the HTTP client to also cache the result for one hour.
* All requests to GitHub are rate limited using a [token bucket algorithm](https://en.wikipedia.org/wiki/Token_bucket) to max. 25 requests per second (burst: 100 requests).
* You can adjust these limits using flags.
  Use `masquerade -help` to learn more about all available flags.
