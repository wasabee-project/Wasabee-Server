[![LICENSE](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GoReportCard](https://goreportcard.com/badge/wasabee-project/Wasabee-Server)](https://goreportcard.com/report/wasabee-project/Wasabee-Server)
[![GoDoc](https://godoc.org/github.com/wasabee-project/Wasabee-Server?status.svg)](https://godoc.org/github.com/wasabee-project/Wasabee-Server)
![GitHub issues](https://img.shields.io/github/issues/wasabee-project/Wasabee-Server.svg)
[![Build Status](https://travis-ci.com/wasabee-project/Wasabee-Server.svg?branch=master)](https://travis-ci.com/wasabee-project/Wasabee-Server)
[![Coverage Status](https://coveralls.io/repos/github/wasabee-project/Wasabee-Server/badge.svg?branch=master)](https://coveralls.io/github/wasabee-project/Wasabee-Server?branch=master)

[IITC Plugin](https://github.com/pcaddict08/Wasabee-IITC)

# Wasabee-Server 
### The Server-Side component for the Wasabee Project tools for Enlightened Agents.

## But... OPSEC!!!!? Tools!!!

RES have RESwue. Every RES agent who wants it has access to it. ENL has great tools too. But ours are hidden away in silos, locked up and only shown to trained and certified operators--who aren't even allowed to mention that they exist. A tool that no one can use is useless. These secure ENL tools are good. But they do not give us a strategic advantage because (1) no one can use them and (2) RES have everything we do (maybe not in the same form, but every feature we have, they have too). We are not going to leak the ENL tools. Rather, we are providing a tool for ENL agents to use that doesn't require operator training, a high level standing in the ENL pantheon, or a blood test to make sure you bleed green.  It just requires the desire to make everything green. 

## But... opsec? My data?

Access to your data is controlled by teams you define. By default each op gets a unique team, but you can transfer multiple ops to the same team. Only your team members can see your op plans and locations. Each agent controls their own status (active/inactive) on the team. Team owners control who is on the teams. Trust is always bidirectional (with a minor qualification for teams linked to open .rocks communities).

## My personal data, GDPR and such!

We do not store your real name, email address, or any personal identifiying information. We do store your GoogleID, the EnlID that V creates (if you use V), and your agent name (if we have it). If you configure Telegram, we store your telegram ID as well.

We do not retain historical location data. The only data point stored is your most recent check-in. Location data older than 3 hours is considered stale and removed.

## But... how do you make sure that only ENL agents use this.

RES have RESwue. They techincally _could_ use this (minimally). But why would they? Why whould they trust us?

The "good" features require agent verifcation. We verify agent information with trusted ENL providers V and .rocks. We observe negative agent status (aka RES/SMURF). If an agent is not verified at V or rocks, they are displayed as unverified. You can add unverified agents to your teams if you want, that's up to you. We don't force people to use V or rocks. They are helpful tools, not systems that control.

## I already use use an enl.rocks community to manage my telegram channel. I don't want to manage a second list of users. 

You are in luck! You can link a team to an exsiting enl.rocks community. Any changes in the community will be reflected in the team.

You can also configure the team to push changes back to your .rocks community.

You can configure uni-directional or bi-directional linking, depending on your needs.

## I use V teams ...

Wasabee can pull team data from V if you register your V api key with us. We cannot manage V teams due to limitations in the V API. We hope we are able to get V to add the necessary API for us.

## Our group uses GroupMe/Slack/Hangouts/AIM/ICQ/IRC...

Look through the Telegram code. Adding support for your favorite system probably shouldn't be too hard. Probably a couple files, few lines of code and you'll have the basics.  Then you send it back to us in a pull request and the beauty of open source in action causes Wasabee to gain more features.

## I don't want to use your app for sharing my location data.

You can send your location via the telegram bot.

You can send your location via the web interface.

You can send your location via the Wasabee-IITC in your IITC program.

You don't have to opt into sharing your location. You lose some functionalty, but location based notifications is only one part of this tool.

## Why not just use <insert tool here> ? Why did you "reinvent the wheel"? ... "such a waste of developer time" ...

Because we like building our own tools.  When we are not smashing blue, and making the world green, we need something to keep our Ingress brains churning.  Genetic diversity is a good thing. Not everyone likes Coke, some pople like tea, coffee, beer or water. 

## But ... opsec, why is the code open?

We believe that having the code open means people looked at it. People will find bugs and hopfully fix them. New features will be added and new ways to solve problems will be shared. ENL have previoulsy completely decompiled/deobfuscated the RESwue client. All the attempts to hide the code was wasted effort. We have little doubt they have seen or heard of most of the ENL "secret" tools (People flip sides). Hiding code does not make it secure. API endpoint probing tools exist. Obscurity is not security, it only makes life harder on the tool maintainers, which can actaully make the code less secure because it is harder to audit. 

Yes, anyone can run this server software. That does not give everyone access to all the ENL data. You would still need API keys for V and rocks. To get those you need to be a trusted ENL agent. It does run (well, it ought to, and probably needs to be tested that way) without V and rocks support enabled. Do you have moral objections to V? Do you hate the rocks people? That's fine. Run your own instance. Disable the parts you don't like. Keep your data on a private server. You do not have to trust us or them to use these tools. We don't have to trust you to let you use the tools. There is no strategic advantage to be had in secrecy when the other side already has a full toolset available to every agent. 

Yes, RES can review all our code and look for vulnerabilities. We hope they do (they won't, unless they just don't have anything else to do with their lives...). We believe that open code is better. Problems get seen, and solved, much more quickly than with closed code.

## Wouldn't (my favorite database) be a better choice than MariaDB/MySQL?

We are old dudes working with SQL since the 1990s. It is what we know. Other backends may be cool and if you want to get another one in, code it up.  Show us it is better than what we have now.

## Go? I like python/C/COBOL/perl/php5/node.js

Opposite answer to the above... When this project started we didn't know Go. We used this project to teach ourselves Go. Most of us are old C hacks, python people, Javascript heads, or php dudes.. We have grown to like Go, it enabled us to make very rapid progress with this project.  Go with it.

## INSTALL
1. Install and configure MySQL or MariaDB
https://mariadb.com/kb/en/library/where-to-download-mariadb/

1.1 Create a database (suggested: wasabee)
https://mariadb.com/kb/en/library/create-database/

1.2 Create a user for that database (suggested: wasabee@localhost)
https://mariadb.com/kb/en/library/create-user/

1.3 GRANT the new user full privileges to that database
https://mariadb.com/kb/en/library/grant/
Tables will be automatically created on the first start-up.

2. Install Go
https://golang.org/doc/install

3. Install git
https://www.git-scm.com/book/en/v2/Getting-Started-Installing-Git

4. Use git to checkout the frontend directory; use go to get wasabee and all dependencies
```
mkdir wasabee; cd wasabee 
git clone https://github.com/wasabee-project/Wasabee-Server/frontend
setenv GOPATH ~/go
go get github.com/wasabee-project/Wasabee-Server
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

#install certificates as wasabee.fullchain.pem and wasabee.key
```

8. Get a GoogleAPI client ID and secret
https://developers.google.com/identity/protocols/OAuth2
https://developers.google.com/identity/protocols/OAuth2WebServer

8.1 Get your V API key (if you want V support)
8.2 Get your .Rocks API key (if you want .Rocks support) (gfl)
8.3 Get your Telegram API key (if you want telegram support)

9. Configure your environment (don't just copy-and-paste this, tweak for your setup!)
9.1 copy wasabee-example.json to wasabee.json
9.2 fill in the fields marked with "..."

10. Start the processes
```
$GOPATH/bin/wasabee & ; $GOPATH/bin/wasabee-reaper &
```
