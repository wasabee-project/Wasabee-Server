[![LICENSE](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GoReportCard](https://goreportcard.com/badge/cloudkucooland/WASABI)](https://goreportcard.com/report/cloudkucooland/WASABI)
[![GoDoc](https://godoc.org/github.com/cloudkucooland/WASABI?status.svg)](https://godoc.org/github.com/cloudkucooland/WASABI)
![GitHub issues](https://img.shields.io/github/issues/cloudkucooland/WASABI.svg)
[![Build Status](https://travis-ci.com/cloudkucooland/WASABI.svg?branch=master)](https://travis-ci.com/cloudkucooland/WASABI)
[![Coverage Status](https://coveralls.io/repos/github/cloudkucooland/WASABI/badge.svg?branch=master)](https://coveralls.io/github/cloudkucooland/WASABI?branch=master)

# WASABI
### The Server-Side component for WASABI and related Ingress tools for Enlightened Agents.

## But... OPSEC!!!!? Tools!!!

RES have RESwue. Every RES agent who wants it has access to it. ENL has great tools too. But ours are hidden away in silos, locked up and only shown to trained and certified operators--who aren't even allowed to mention that they exist. A tool that no one can use is useless. These secure ENL tools are good. But they do not give us a strategic advantage because (1) no one can use them and (2) RES have everything we do (maybe not in the same form, but every feature we have, they have too). We are not going to leak the ENL tools. Rather, we are providing a tool for ENL agents to use that doesn't require operator training, a high level standing in the ENL pantheon, or a blood test to make sure you bleed green.  It just requires the desire to make everything green. 

## But... opsec? My data?

If you use the "simple" mode of transfering draws, your data is enccoded in IITC, sent to the server and never decoded. It is stored encrypted in such a way that it is only decryptable upon request. The URL is the key. We don't know the URL. If you lose the URL, the draw is inaccessible. 

If you use the normal method, many more features will be enabled (proximity to marker notifications). Access to your data is controlled by teams you define. By default each op gets a unique team, but you can transfer multiple ops to the same team. Only your team members can see your op plans and locations. Each agent controls their own status (active/inactive) on the team. Team owners control who is on the teams. Trust is always bidirectional (with a minor qualification for teams linked to open .rocks communities).

## My personal data, GDPR and such!

We do not store your real name, email address, or any personal identifiying information. We do store your GoogleID, the EnlID that V creates (if you use V), and your agent name (if you use V or rocks). If you configure Telegram (either here or at .rocks), we store your telegram ID as well. As support is added for other messaging systems, we will store any messaging identifiers you opt-in to.

We do not retain historical location data. The only data point stored is your most recent check-in. If, at the end of your op, you use the web interface to set the values to something absurd (0,0 is handy), we will not know where you are or where you've been. Location data older than 3 hours is considered stale and removed.

## But... how do you make sure that only ENL agents use this.

RES have RESwue. They techincally _could_ use this (minimally). But why would they? Why whould they trust us?

The "good" stuff/features require agent verifcation. We verify agent information with trusted ENL providers V and .rocks. We observe negative agent status (aka RES/SMURF). If an agent is not verified at V or rocks, they are displayed as unverified. You can add unverified agents to your teams if you want, that's up to you. We don't force people to use V or rocks. They are helpful tools, not systems that control.

## I already use use an enl.rocks community to manage my telegram channel. I don't want to manage a second list of users. 

You are in luck! You can link a team to an exsiting enl.rocks community. Any changes in the community will be reflected in the team.

You can also configure the team to push changes back to your .rocks community.

You can configure uni-directional or bi-directional linking, depending on your needs.

## I use V teams ...

That's cool also. It wouldn't be hard to add support for manually pulling team data across. V does not have push notifications, nor does it have an API to manage team members, so it would require operator action to sync the changes.  If you are insterested in adding this, open a pull request.

## Our group uses GroupMe/Slack/Hangouts/AIM/ICQ/IRC...

Look through the Telegram code. Adding support for your favorite system probably shouldn't be too hard. Probably a couple files, few lines of code and you'll have the basics.  Then you send it back to us in a pull request and the beauty of open source in action causes WASABI to gain more features.

## I don't want to use your app for sharing my location data.

We use OwnTracks now, we are working on our own app to handle this. We are using OwnTracks because it is also open source. But you don't have to trust it.

We can pull from the RAID location store if you give us permission.

You can send your location via the telegram bot.

You can send your location via the web interface.

You don't have to opt into sharing your location. You lose some functionalty, but location based notifications is only one part of this tool.

Maybe we will support glympse someday, if someone really wants it.

## Why not just use <insert tool here> ?

Because we like building our own tools.  When we are not smashing blue, and making the world green, we need something to keep our Ingress brains churning.  Genetic diversity is a good thing.

## But ... opsec, why is the code open?

We believe that having the code open means people looked at it. People will find bugs and hopfully fix them. New features will be added and new ways to solve problems will be shared. ENL have previoulsy completely decompiled/deobfuscated the RESwue client. All the attempts to hide the code was wasted effort. We have little doubt they have seen or heard of most of the ENL "secret" tools (People flip sides). Hiding code does not make it secure. API endpoint probing tools exist. Obscurity is not security, it only makes life harder on the tool maintainers, which can actaully make the code less secure because it is harder to audit. 

Yes, anyone can run this server software. That does not give everyone access to all the ENL data. You would still need API keys for V and rocks. To get those you need to be a trusted ENL agent. It does run (well, it ought to, and probably needs to be tested that way) without V and rocks support enabled. Do you have moral objections to V? Do you hate the rocks people? That's fine. Run your own instance. Disable the parts you don't like. Keep your data on a private server. You do not have to trust us or them to use these tools. We don't have to trust you to let you use the tools. There is no strategic advantage to be had in secrecy when the other side already has a full toolset available to every agent. 

## RABBITMQ/MQTT?

We haven't used it yet. We have plans to look at optionally integrating a message broker.

## Wouldn't (my favorite database) be a better choice than MariaDB/MySQL?

We are old dudes working with SQL since the 1990s. It is what we know. Other backends may be cool and if you want to get another one in, code it up.  Show us it is better than what we have now.

## Go? I like python/C/COBOL/perl/php5/node.js

Opposite answer to the above... When this project started we didn't know Go. We used this project to teach ourselves Go. Most of us are old C hacks, python people, Javascript heads, or php dudes.. We have grown to like Go, it enabled us to make very rapid progress with this project.  Go with it.

## INSTALL
1. Install and configure MySQL or MariaDB
https://mariadb.com/kb/en/library/where-to-download-mariadb/

1.1 Create a database (suggested: wasabi)
https://mariadb.com/kb/en/library/create-database/

1.2 Create a user for that database (suggested: wasabi@localhost)
https://mariadb.com/kb/en/library/create-user/

1.3 GRANT the new user full privileges to that database
https://mariadb.com/kb/en/library/grant/
Tables will be automatically created on the first start-up.

2. Install Go
https://golang.org/doc/install

3. Install git
https://www.git-scm.com/book/en/v2/Getting-Started-Installing-Git

4. Use git to checkout the frontend directory; use go to get wasabi and all dependencies
```
mkdir WASABI ; cd WASABI
git clone https://github.com/cloudkucooland/WASABI/frontend
setenv GOPATH ~/go
go get github.com/cloudkucooland/WASABI
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

8.1 Get your V API key (if you want V support)
8.2 Get your .Rocks API key (if you want .Rocks support)
8.3 Get your Telegram API key (if you want telegram support)

9. Configure your environment (don't just copy-and-paste this, tweak for your setup!)
```
setenv GOPATH ~/go
setenv DATABASE "wasabi:password@tcp(localhost)/wasabi"
setenv ROOT_URL "https://wasabi.example.com:8443"
# this is the port to listen on, :8443 is the suggested value
setenv HTTPS_LISTEN ":8443"
setenv GOOGLE_CLIENT_ID "--SOMETHING--SOMETHING--SOMETHING--.apps.googleusercontent.com"
setenv GOOGLE_CLIENT_SECRET "--SOMETHING--SOMETHING--"
# The session-key is what encrypts the authentication cookie, it can be completely random, but must be 32 characters long
setenv SESSION_KEY "!-rand0m-32-_char-sTring-blah-xy"
setenv TELEGRAM_API_KEY "--SOMETHING--"
setenv VENLONE_API_KEY "--SOMETHING--"
setenv ENLROCKS_API_KEY "--SOMETHING--"
# enable DEBUG to see verbose output
#setenv DEBUG 0
# enable the poller if you need JEAH/RAID support
#setenv VENLONE_POLLER 0

```

10. Start the processes
```
$GOPATH/bin/wasabi & ; $GOPATH/bin/wasabi-reaper &
```
