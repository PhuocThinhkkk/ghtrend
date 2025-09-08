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

## Installation (for linux, idk how to do it on Windows )

Clone this repository:

```
git clone https://github.com/PhuocThinhkkk/ghtrend.git
```

Enter the directory:

```bash
cd ghtrend
```

```bash
chmod +x install.sh
./install.sh
```

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


