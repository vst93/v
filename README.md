# v
Gadgets under the terminal

### Install & Uninstall
``` bash
# brew 
## install 
brew install vst93/tap/v
## uninstall 
brew uninstall v

# shell 
## install 
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/vst93/v/refs/heads/main/cmd/install.sh)"

```


### Usage Examples

``` bash    
# json2excel 
$ v json2excel -i 'xxxx/xxxx/xxx.xx'  -k 'data.list'
$ v json2excel -c '{xxxxx}'
$ curl xxxx | v json2excel -k 'data.list'

```