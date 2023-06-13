package handlers

var Logo string = "                                                    ,,                          \n .M\"\"\"bgd                                         `7MM                    mm\n,MI    \"Y                                           MM                    MM\n`MMb.     `7MMpMMMb.   ,6\"Yb.  `7MMpdMAo. ,pP\"Ybd   MMpMMMb.   ,pW\"Wq.  mmMMmm  \n  `YMMNq.   MM    MM  8)   MM    MM   `Wb 8I   `\"   MM    MM  6W'   `Wb   MM\n.     `MM   MM    MM   ,pm9MM    MM    M8`YMMMa.    MM    MM  8M     M8   MM\nMb     dM   MM    MM  8M   MM    MM   ,AP L.   I8   MM    MM  YA.   ,A9   MM\nP\"Ybmmd\"  .JMML  JMML.`Moo9^Yo.  MMbmmd'  M9mmmP' .JMML  JMML. `Ybmd9'    `Mbmo\n                                 MM\n                               .JMML.\n"

var CommandList string = `
1. To list available commands, type ` + "\x1b[33m'snp -h'\x1b[0m." + `
2. To add a project, type ` + "\x1b[33m'snp -a site.ru'\x1b[0m." + `
In the next steps, you will be prompted to enter:
    - project name;
    - port (21 FTP, 22 SFTP);
    - server address;
    - login S\\FTP;
    - password S\\FTP;
    - bd login;
    - bd password;
    - resource root directory;
    - directory for saving the snapshot on the local machine;
    - backup frequency in cron task syntax.
3. To change project settings, type ` + "\x1b[33m'snp -e *project ID* *data field name* *new value*'\x1b[0m." + ` Settings can be changed in the config.json file.
4. To get a list of all added projects, type ` + "\x1b[33m'snp -l'\x1b[0m." + `
5.To remove the project from the list with all previously created snapshots, enter ` + "\x1b[33m'snp -d *project ID*'\x1b[0m." + `
6. To get a list of all settings for a specific project, type ` + "\x1b[33m'snp -s *project ID*'\x1b[0m." + `
7. To create a snapshot of a specific project, type ` + "\x1b[33m'snp -c *project ID*'\x1b[0m." + `
8. To create snapshots of all added projects, type ` + "\x1b[33m'snp -cl'\x1b[0m." + `
9. To get a list of available backups for a specific project, type ` + "\x1b[33m'snp -ls *project ID*'\x1b[0m." + `
10. To create a snapshot of a specific project, type ` + "\x1b[33m'snp -gs *project ID* *snapshot ID*'\x1b[0m." + `
11. To get the files or directory from the selected snapshot, type ` + "\x1b[33m'snp -gsf *project ID* *snapshot ID* *directory*'\x1b[0m." + `
12. To compare two snapshots, type ` + "\x1b[33m'snp -sc *project ID* *snapshot ID* *snapshot ID'\x1b[0m."
