package handlers

var Logo string = "                                                    ,,                          \n .M\"\"\"bgd                                         `7MM                    mm\n,MI    \"Y                                           MM                    MM\n`MMb.     `7MMpMMMb.   ,6\"Yb.  `7MMpdMAo. ,pP\"Ybd   MMpMMMb.   ,pW\"Wq.  mmMMmm  \n  `YMMNq.   MM    MM  8)   MM    MM   `Wb 8I   `\"   MM    MM  6W'   `Wb   MM\n.     `MM   MM    MM   ,pm9MM    MM    M8`YMMMa.    MM    MM  8M     M8   MM\nMb     dM   MM    MM  8M   MM    MM   ,AP L.   I8   MM    MM  YA.   ,A9   MM\nP\"Ybmmd\"  .JMML  JMML.`Moo9^Yo.  MMbmmd'  M9mmmP' .JMML  JMML. `Ybmd9'    `Mbmo\n\x1b[3mv.0.0.1\x1b[0m                          MM\n                               .JMML."

var CommandList string = `
Options:
` + "\x1b[33m-h --help\x1b[0m" + `	to list available commands.
` + "\x1b[33m-a --add\x1b[0m" + `	to add a project. Required arguments: #domain#.
In the next steps, you will be prompted to enter:
	- project name;
	- port (21 FTP, 22 SFTP);
	- server address;
	- login S/FTP;
	- password S/FTP;
	- bd login;
	- bd password;
	- resource root directory;
	- directory for saving the snapshot on the local machine;
	- backup frequency in cron task syntax.
` + "\x1b[33m-e --edit\x1b[0m" + `	to change project settings. Settings can be changed in the config.json file. Required arguments: #project ID# #data field name# #new value#.
` + "\x1b[33m-l --list\x1b[0m" + `	to get a list of all added projects.
` + "\x1b[33m-d --delet\x1b[0m" + `	to remove the project from the list with all previously created snapshots. Required arguments: #project ID#.
` + "\x1b[33m-s --settings\x1b[0m" + `	to get a list of all settings for a specific project. Required arguments: #project ID#.
` + "\x1b[33m-c --create\x1b[0m" + `	to create a snapshot of a specific project. Required arguments: #project ID#.
` + "\x1b[33m-cl\x1b[0m" + `		to create snapshots of all added projects.
` + "\x1b[33m-ls\x1b[0m" + `		to get a list of available backups for a specific project. Required arguments: #project ID#.
` + "\x1b[33m-gs\x1b[0m" + `		to create a snapshot of a specific project. Required arguments: #project ID# #snapshot ID#.
` + "\x1b[33m-gsf\x1b[0m" + `		to get the files or directory from the selected snapshot. Required arguments: #project ID# #snapshot ID# #directory#.
` + "\x1b[33m-sc\x1b[0m" + `		to compare two snapshots. Required arguments: #project ID# #snapshot ID first# #snapshot ID second#.`
