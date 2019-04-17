[![LICENSE](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GoReportCard](https://goreportcard.com/badge/cloudkucooland/WASABI)](https://goreportcard.com/report/cloudkucooland/WASABI)
[![GoDoc](https://godoc.org/github.com/cloudkucooland/WASABI?status.svg)](https://godoc.org/github.com/cloudkucooland/WASABI)
![GitHub issues](https://img.shields.io/github/issues/cloudkucooland/WASABI.svg)
[![Build Status](https://travis-ci.com/cloudkucooland/WASABI.svg?branch=master)](https://travis-ci.com/cloudkucooland/WASABI)

# WASABI
### The Server-Side component for WASABI and related Ingress tools.

## But... OPSEC!!!!? Tools!!!

RES have RESwue. Every RES agent who wants it has access to it. ENL has great tools too. But ours are hidden away in silos, locked up and only shown to trained and certified operators--who aren't even allowed to mention that they exist. A tool that no one can use is useless. These secure ENL tools are good. But they do not give us a strategic advantage because (1) no one can use them and (2) RES have everything we do (maybe not in the same form, but every feature we have, they have too). We are not going to leak the ENL tools. Rather, we are going to provide a tool to ENL agents who want access to something that doesn't require operator training, 64 vouches on V and a blood test to make sure you bleed green. 

## But... opsec? My data?

If you use the "simple" mode of transfering draws, your data is encoded in IITC, sent to the server and never decoded. It is stored encrypted in such a way that it is only decryptable upon request. The URL is the key. We don't know the URL. If you lose the URL, the draw is inaccessible.

If you use the normal method, many more features will be enabled (proximity to marker notifications). You control access to your data by your teams. By default each op gets a unique team. But you can transfer multiple ops to the same team. Only your team members can see your op plans and locations. Each agent controls their own status (active/inactive) on teams. Team owners control who is on the teams. Trust is always bidirectional (*with a minor qualification for teams linked to open .rocks communities).

## My personal data!

We do not store your real name, your email address or any primary identifiying information. We do store your GoogleID, the EnlID that V creates (if you use V) and your agent name (if you use V or rocks). If you configure Telegram (either here or at .rocks), we store your telegram ID as well. As support is added for other messaging systems, we will store any messaging identifiers you opt-in to.

We do not store historical location data. The only data point we have stored is your most recent check-in. If, at the end of your op, you use the web interface to set the values to something absurd (0,0 is handy), we will not know where you are or where you've been. All stale location data (older than 3 hours) is removed.

## But... how do you make sure that only ENL agents use this.

RES have RESwue. They _can_ use this (minimally). But why would they? Why whould they entrust their op data to us?

We verify agent information with both V and .rocks. We observe the blacklisted/smurf/flagged/banned. If an agent is not verified at V or rocks, they are displayed as unverified. You can add unverified agents to your teams if you want, that's up to you. We don't force people to use V or rocks. They are helpful tools, not systems that control.

## I use an enl.rocks community to manage my telegram channel. I don't want to manage a second list of users. 

Excellent. You can link a team to an enl.rocks community to a team. Any changes in the community will instantly be made in the team.

You can configure the team to push changes to .rocks as well. Adding currently works, deleting does not (yet).

## I use a V team ...

Cool. It wouldn't be hard to add support for manually pulling team data across if this is a real use-case. V does not have push notifications, nor does it have an API to manage team members, so it would require operator action to sync the changes.

## Our group uses GroupMe/Slack/Hangouts/AIM/ICQ/IRC...

Look through the Telegram code. Adding support for your favorite system probably won't be too hard. Probably 2 files, 300 lines of code and you'll have basic functionality. Send a patch.

## I don't want to use your app for sharing my location data.

We use OwnTracks now, our own app will be forthcoming. OwnTracks is open source. But you don't have to trust it.

We can pull from the RAID location store if you give us permission.

You can send your location via the telegram bot.

You can send your location via the web interface.

You don't have to opt into sharing your location. You lose some functionalty, but location based notifications is only one part of this tool.

Maybe we will support glympse someday, if someone really wants it.

## Why not just use ... ?

Because we like building our own tools. Genetic diversity is a good thing.

## But ... opsec, why is the code open?

I believe that open code gets looked at, bugs found, problems solved. ENL have completely decompiled/deobfuscated the RESwue client--all their attempts to hide the code was wasted effort. I have no doubt they have seen most of our "secret" tools (flips, even high-level operators, happen). Hiding code does not make it secure. API endpoint probing tools exist. Obscurity is not security, it only makes life harder on the tool maintainers, which can actaully make the code less secure because it is harder to audit. 

Yes, anyone can run this server software. That does not give everyone access to all the ENL data. You will need API keys for V and rocks. To get those you need to be a trusted ENL agent. It does run (well, it ought to, I really should test that) without V and rocks support enabled. Do you have moral objections to V? Do you hate the rocks people? That's fine. Run your own instance. Disable the parts you don't like. Keep your data on a private server. You do not have to trust us to use these tools. We don't have to trust you to let you use the tools. There is no strategic advantage to be had in secrecy when the other side already has a full toolset available to every agent. 

## MQTT?

Not used yet. I have plans.

## Wouldn't (my favorite database) be a better choice than MariaDB/MySQL?

I've been working with MySQL since the 1990s. It is what I know. (mfd) may be cool, get to porting if you want to prove it is better.

## Go? I like python/C/COBOL/perl/php5/node.js

Opposite answer to the above... I didn't know Go when I started this. I learned Go. I really like Go now. That's saying a lot because I'm a crusty old C hack. I know C, PHP, Python ... but this just came together very quickly in Go. Go with it.

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

3.1 Install the various Oauth, MariaDB and Telegram API bindings
```
go get -v -u golang.org/x/oauth2/google golang.org/x/crypto/scrypt github.com/urfave/cli github.com/unrolled/logger github.com/op/go-logging github.com/gorilla/sessions github.com/gorilla/mux github.com/go-telegram-bot-api/telegram-bot-api github.com/go-sql-driver/mysql
```

4. Use git to checkout the frontend and cmd directories
```
go get github.com/cloudkucooland/wasabi/cmd/wasabi
mkdir WASABI ; cd WASABI
git clone https://github.com/cloudkucooland/WASABI/frontend
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
#install certificates as WASABI.fullchain.pem and WASABI.key
```
```
NB: you can point WASABI to your ACME directory, so long as both files are named correctly and in the same directory
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
setenv TELEGRAM_API_KEY "--SOMETHING--"
setenv VENLONE_API_KEY "--SOMETHING--"
setenv DEBUG 0
```

10. $GOPATH/bin/wasabi
