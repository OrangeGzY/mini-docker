package nsenter

/*
#define _GNU_SOURCE
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>


__attribute__( (constructor) ) void enter_namespace(void) {


    char *mydocker_pid ;
    mydocker_pid = getenv("mydocker_pid");
    if(mydocker_pid){

    }else{
		fprintf(stdout, "missing mydocker_pid env skip nsenter");
        return;
    }
    char *mydocker_cmd;
    mydocker_cmd = getenv("mydocker_cmd");
  if (mydocker_cmd) {
		//fprintf(stdout, "got mydocker_cmd=%s\n", mydocker_cmd);
	} else {
		fprintf(stdout, "missing mydocker_cmd env skip nsenter");
		return;
	}

    int i;
    char nspath[0x1000];
    char *namespace[] = {
        "ipc",
        "uts",
        "net",
		"pid",
        "mnt",

    };

    for( i=0 ; i<5;i++){

        // Open and Setns the namespace one by one
        sprintf(nspath , "/proc/%s/ns/%s" , mydocker_pid , namespace[i]);
        int fd = open(nspath , O_RDONLY);


		if (setns(fd, 0) == -1) {
			fprintf(stderr, "setns on %s namespace failed: %s\n", namespace[i], strerror(errno));
		} else {
			fprintf(stdout, "switch namespace [%s] succeeded\n", namespace[i]);
		};
        close(fd);
    }

    // After we enter the namespace, we execute the command in the target namespace.
    int ret = system(mydocker_cmd);
    exit(0);
    return ;

}
*/
import "C"
