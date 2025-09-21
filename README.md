<a id="readme-top"></a>

<div align="center">
  <h1 align="center">ghtrend</h3>

  <p align="center">
  Explore Github trending directly from your terminal
    <br />
    <br />
  </p>
</div>

<br><br>

## Demo
![Demo](assets/demo.gif)

## Requirements
- Go **1.24+**
- Git

## Installation

### Option 1 — Install with Go (Linux, macOS, Windows)
Make sure Go is installed, then run:

```bash
go install github.com/PhuocThinhkkk/ghtrend@latest
````
This will build and install ghtrend into your $GOBIN (default: $HOME/go/bin).
On Windows, it goes into %USERPROFILE%\go\bin.

### Option 2 — Install via script (Linux/macOS only)
#### Quick install (curl):
```bash
curl -fsSL https://raw.githubusercontent.com/PhuocThinhkkk/ghtrend/main/install.sh | bash
````

#### Manual install:
````bash
git clone https://github.com/PhuocThinhkkk/ghtrend.git
cd ghtrend
chmod +x install.sh
./install.sh
````


After installing, run:
```bash
ghtrend
```
## Usage

**Basic command:**
```bash
ghtrend
```

**Help command:**
```bash
ghtrend --help
```

**Get top 5 trending Go repositories:**

```bash
ghtrend --lang go --limit 5
```
**Show trending for the past week:**

```bash
ghtrend --since weekly
```
**Disable cache and fetch fresh data:**

```bash
ghtrend --no-cache
```


