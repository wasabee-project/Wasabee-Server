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
{{define "edit"}}
{{define "index"}}
{{define "me"}}
{{define "status"}}
```

## not required, used to make life easier in the required templates
```
layout.html:{{define "styles"}}
layout.html:{{define "nav"}}
layout.html:{{define "footer"}}
```

## REQUIRED Telegram Templates
```
{{define "default"}}
{{define "help"}}
{{define "InitOneFail"}}
{{define "InitOneSuccess"}}
{{define "InitTwoFail"}}
{{define "InitTwoSuccess"}}
{{define "TeamStateChange"}}
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

