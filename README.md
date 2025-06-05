# Go Apps

This project does **not** compile with Go 1.20 or newer due to breaking changes in:
    - `quic-go` (`github.com/fkwhite/Quic_GO`)

## Project Structure

    .
    ├── go_client/             # Client application (Go)
    ├── go_server/             # Server application (Go)
    ├── conf_quic.json         # Common configuration file for both apps
    ├── LOGS_congestionWindow/ # Output folder for congestion window logs
    ├── tmp/                   # Output folder for temporary results
    ├── Makefile               # Automation script for install/build/run
    └── README.md              # This file


To install Go 1.19.13 manually:
```bash
cd /tmp
wget https://go.dev/dl/go1.19.13.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.19.13.linux-amd64.tar.gz
```
Update your environment:

```bash
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```


To execute apps:

- Client:

```bash
./go_client/go_client ./conf_quic.json 
```

- Server:

```bash
 ./go_server/server ./conf_quic.json 
```


Results are stored at folders: tmp and LOGS_congestionWindow