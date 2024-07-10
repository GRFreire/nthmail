# Nothing Mail

A temporary email service

## TODO

 - Restart when either mail or web server dies
 - Do not store the raw mail data in the DB, maybe use block storage (the provider can be a disk provider at first)
 - Cache subject parsed from email. Then when listing the email it is not necessary to parse all mails and retrieve them.
 - Cache in general?
