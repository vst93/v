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
# json2excel - convert json data to excel file
$ v json2excel -i 'xxxx/xxxx/xxx.xx'  -k 'data.list'
$ v json2excel -c '{xxxxx}'
$ curl xxxx | v json2excel -k 'data.list'

# tt - provides mutual conversion of timestamp and date
$ v tt
`1641038400`
$ v tt -m 
`1765533341652`
$ v tt '2022-01-01 12:00:00'
`1641038400`
$ v tt 1641038400
`2022-01-01 20:00:00`


```