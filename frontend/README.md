# Load order
Template files are loaded in the following order:
```
Master
[languages]
```
Any templates in the languages overwrite those in the master.

It is safe to copy a file from master into a language and edit it in each language

## REQUIRED HTTPS Templates
```
edit
index
me
status
```

## not required, used to make life easier in the required templates
```
styles
nav
footer
```

## REQUIRED Telegram Templates
```
default
help
InitOneFail
InitOneSuccess
InitTwoFail
InitTwoSuccess
TeamStateChange
```

Required templates can call any number of non-required templates, a non-required template cannot call another template

### Functions
```
tbd
```

### Variables
```
The variables available depend on the calling model. Consult the datatypes at the top of the relevant model for more info
```

