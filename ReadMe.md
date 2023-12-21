# Forum Crawler

A crawler application reading forum's topics (threads) and saving them into an 
_SQL_ database.

Currently, this crawler can only save names of topics (threads). 

List of forums must be created manually and stored in a file having the _CSV_ 
format, where first column is `forum_id`, second column is `forum_name`. 

## Usage
CLI Arguments 
> program.exe [SettingsFile] [Action] [Object] [Parameters]

Actions:
* init
* refresh

Objects:
* forums
* forum_topics
* all_topics

Parameters:
* forum_id
* start_forum_id
* forum_page
* first_pages

Various combinations of actions and objects support different sets of 
parameters.

Parameters are written using key-value pairs separated by comma (`,`) symbols.  
Key and value are separated by an equality (`=`) sign.   
Example:  
> x=1,y=2

Empty (void) parameters are represented as a hyphen (`-`) symbol.

## Database

This crawler saves data into a _MySQL_ database.  

Big _SQL_ queries of the `init` action are also saved into a temporary folder 
for each forum for debugging purposes. This can be useful when queries can not 
be used immediately due to some errors in the process of data saving, e.g. some 
IDs may be duplicated and this can raise an error.

By default, indices are not created. Separate _SQL_ scripts for creation of 
indices are available in the `scripts` folder.
