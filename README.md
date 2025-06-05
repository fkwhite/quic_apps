# Go Apps

This project does **not** compile with Go 1.20 or newer due to breaking changes in:
    - `quic-go` (`github.com/fkwhite/Quic_GO`)

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



