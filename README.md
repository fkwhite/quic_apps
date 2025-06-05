# Go Apps

This project contains a client-server application using QUIC based on a modified version of quic-go.


Important Compatibility Note:
This project does not compile with Go 1.20 or newer due to breaking changes in quic-go.
Please install and use Go 1.19.13 to build and run the application.

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


## Project Structure

    .
    ├── go_client/               # Client application (written in Go)
    ├── go_server/               # Server application (written in Go)
    ├── conf_quic.json           # Configuration file related to QUIC and data transmission
    ├── conf_Scheduler.json      # Configuration file for scheduling behavior (used by both apps)
    ├── LOGS_congestionWindow/   # Output folder for QUIC congestion window logs
    ├── tmp/                     # Output folder for buffer results and app traces
    ├── Makefile                 # Automation script for installation, build, and execution
    └── README.md                # Project documentation




To execute apps:

- Client:

```bash
./go_client/go_client ./conf_quic.json 
```

- Server:

```bash
 ./go_server/server ./conf_quic.json 
```

## Output and Logs

Results will be saved in the following folders:

-  `tmp/`  — Contains results from the buffer as well as traces of both apps

-  `LOGS_congestionWindow/`  — Contains logs related to QUIC congestion window behavior.