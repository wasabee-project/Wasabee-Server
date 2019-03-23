[![GoReportCard](https://goreportcard.com/badge/cloudkucooland/PhDevBin)](https://goreportcard.com/report/cloudkucooland/PhDevBin)

# PhDevBin
### The Server-Side component for Phtiv-Draw-Tools and related Ingress tools.

## INSTALL
1. Install and configure MySQL or MariaDB
https://mariadb.com/kb/en/library/where-to-download-mariadb/

1.1 Create a database (suggested: phdev)
https://mariadb.com/kb/en/library/create-database/

1.2 Create a user for that database (suggested: phdev@localhost)
https://mariadb.com/kb/en/library/create-user/

1.3 GRANT the new user full privileges to that database
https://mariadb.com/kb/en/library/grant/
Tables will be automatically created on the first start-up.

2. Install Go
https://golang.org/doc/install

3. Install git
https://www.git-scm.com/book/en/v2/Getting-Started-Installing-Git

3.1 Install the Telegram API bindings
```
go get -u github.com/go-telegram-bot-api/telegram-bot-api
```

4. Use git to checkout the frontend and cmd directories
```
mkdir PhDevBin ; cd PhDevBin
go get github.com/cloudkucooland/PhDevBin
git clone https://github.com/cloudkucooland/PhDevBin/frontend
```

5. Build the eff_large_wordlist.txt file
```
curl -s https://www.eff.org/files/2016/07/18/eff_large_wordlist.txt | sed -r 's/^[0-9]+\t//g' >> eff_large_wordlist.txt
```
--NB, this didn't quite work for me, test and redo, also assumes curl installed--

6. Get a valid certificate using your favorite ACME client
https://letsencrypt.org/docs/client-options/

7. Create the certificate directories
```
mkdir certs
#install certificates as PhDevBin.fullchain.pem and PhDevBin.key
```
```
NB: you can point PhDevBin to your ACME directory, so long as both files are named correctly and in the same directory
TODO: allow different names and the key to be in $CERT_DIR/keys/ as how most ACME clients create them
```

8. Get a GoogleAPI client ID and secret
https://developers.google.com/identity/protocols/OAuth2
https://developers.google.com/identity/protocols/OAuth2WebServer

9. Configure your environment
```
setenv GOPATH ~/go
setenv DATABASE "phdev:@tcp(localhost)/phdev"
setenv ROOT_URL "https://qbin.phtiv.com:8443"
setenv HTTPS_LISTEN ":8443"
setenv GOOGLE_CLIENT_ID "--SOMETHING--SOMETHING--SOMETHING--.apps.googleusercontent.com"
setenv GOOGLE_CLIENT_SECRET "--SOMETHING--SOMETHING--"
setenv SESSION_KEY "!-rand0m-32-_char-sTring-blah-xy"
```

10. Go...
