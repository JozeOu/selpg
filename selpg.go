/*=================================================================

Program name:
	selpg (SELect PaGes)

Purpose:
	Sometimes one needs to extract only a specified range of
pages from an input text file. This program allows the user to do
that.

Author: Joze Ou

===================================================================*/

package main

/*================================= imports =========================*/

import (
	"io"
	"os/exec"
	"bufio"
	"os"
	"fmt"
	flag "github.com/spf13/pflag"
)

/*================================= types =========================*/

type  selpg_args struct {
	start_page int  // 打印第 start_page 页至第 end_page 页
	end_page int
	in_filename string
	page_len int  /* default value, can be overriden by "-l number" on command line */
	page_type string  /* 'l' for lines-delimited, 'f' for form-feed-delimited, default is 'l' */

	print_dest string
}
type sp_args selpg_args

/*================================= globals =======================*/

var progname string /* program name, for error messages */

/*================================= main() ========================*/

func main() {
	sa := sp_args{}

	/* save name by which program is invoked, for error messages */
	progname = os.Args[0]
	
	process_args(&sa)
	process_input(sa)
}

/*================================= process_args() ================*/

func process_args(sa * sp_args) {
	/* 将 flag 绑定到 sa 中的变量 */ 
	flag.IntVarP(&sa.start_page, "start_page", "s", 0, "Input the start page")
	flag.IntVarP(&sa.end_page, "end_page", "e",  0, "Input the end page")
	flag.IntVarP(&sa.page_len, "page_len", "l", 10, "Input the lines per page")
	flag.StringVarP(&sa.print_dest, "print_dest", "d", "", "Input the print dest file")
	flag.StringVarP(&sa.page_type, "page_type", "f", "l", "Input the page type, 'l' for lines-delimited, 'f' for form-feed-delimited. default is 'l'")
	flag.Lookup("page_type").NoOptDefVal = "f" // 设置非必须选项的默认值，实现参数模式要求
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"\nUSAGE: %s -s start_page -e end_page [ -f | -l lines_per_page ]" +
	 		" [ -d dest ] [ in_filename ]\n", progname)
		flag.PrintDefaults()
	}
	
	flag.Parse() // flag 解析，解析函数将会在碰到第一个非 flag 命令行参数时停止
	
	/* os.Args at index 0 is the command name itself (selpg),
	   first actual arg is at index 1,
	   last arg is at index (len(os.Args) - 1) */

	/* check the command-line arguments for validity */
	if len(os.Args) < 3 {	/* Not enough args, minimum command is "selpg -sstartpage -eend_page"  */
		fmt.Fprintf(os.Stderr, "%s: not enough arguments\n", progname)
		flag.Usage()
		os.Exit(1)
	}

	/* handle mandatory args first */

	/* handle 1st arg - start page */
	if os.Args[1] != "-s" {
		fmt.Fprintf(os.Stderr, "%s: 1st arg should be -s start_page\n", progname)
		flag.Usage()
		os.Exit(2)
	}
	
	INT_MAX := 1 << 32
	
	if sa.start_page < 1 || sa.start_page > (INT_MAX - 1) {
		fmt.Fprintf(os.Stderr, "%s: invalid start page %s\n", progname, sa.start_page)
		flag.Usage()
		os.Exit(3)
	}

	/* handle 2nd arg - end page */
	if os.Args[3] != "-e" {
		fmt.Fprintf(os.Stderr, "%s: 2nd arg should be -e end_page\n", progname)
		flag.Usage()
		os.Exit(2)
	}
	
	if sa.end_page < 1 || sa.end_page > (INT_MAX - 1) || sa.end_page < sa.start_page {
		fmt.Fprintf(os.Stderr, "%s: invalid end page %s\n", progname, sa.end_page)
		flag.Usage()
		os.Exit(3)
	}

	/* now handle optional args */

	/* handle page_len */
	if sa.page_len < 1 || sa.page_len > (INT_MAX - 1) {
		fmt.Fprintf(os.Stderr, "%s: invalid page length %s\n", progname, sa.page_len)
		flag.Usage()
		os.Exit(3)
	}
	
	/* handle in_filename */ 
	if len(flag.Args()) == 1 { /* there is one more arg */
		_, err := os.Stat(flag.Args()[0])
		/* check if file exists */
		if err != nil && os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "%s: input file \"%s\" does not exist\n",
					progname, flag.Args()[0]);
			os.Exit(4);
		}

		sa.in_filename = flag.Args()[0]
	}
}

/*================================= process_input() ===============*/

func process_input(sa sp_args) {
	var fin *os.File /* input stream */	
	/* set the input source */
	if len(sa.in_filename) == 0 { // 进行标准输入
		fin = os.Stdin
	} else { // 读取 in_filename 文件
		var err error
		fin, err = os.Open(sa.in_filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not open input file \"%s\"\n",
				progname, sa.in_filename)
			os.Exit(5)
		}
		defer fin.Close()
	}

	/* use  bufio.NewReader() to set a big buffer for fin, for performance */
	bufFin := bufio.NewReader(fin)
	
	var fout io.WriteCloser /* output stream */
	/* set the output destination */
	cmd := &exec.Cmd{}
	if len(sa.print_dest) == 0 { // 进行标准输出
		fout = os.Stdout
	} else { // 将输出流写入 print_dest 文件
		cmd = exec.Command("cat")	
		var err error
		cmd.Stdout, err = os.OpenFile(sa.print_dest, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not open file %s\n",
				progname, sa.print_dest)
			os.Exit(6)
		}

		fout, err = cmd.StdinPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: could not open pipe to file %s\n",
				progname, sa.print_dest)
			os.Exit(6)
		}
		
		cmd.Start()
		defer fout.Close()
	}

	/* begin one of two main loops to print result based on page type */
	var page_ctr int /* page counter */
	var line_ctr int /* line counter */
	if sa.page_type == "l" {
		line_ctr = 0
		page_ctr = 1
		for {
			line,  err := bufFin.ReadString('\n')
			if err != nil {	/* error or EOF */
				break
			}
			line_ctr++
			if line_ctr > sa.page_len {
				page_ctr++
				line_ctr = 1
			}
			if (page_ctr >= sa.start_page) && (page_ctr <= sa.end_page) {
				_, err := fout.Write([]byte(line))
				if err != nil {
					fmt.Println(err)
					os.Exit(7)
				}
		 	}
		}  
	} else {
		page_ctr = 1
		for {
			page, err := bufFin.ReadString('\f')
			if err != nil { /* error or EOF */
				break
			}
			if (page_ctr >= sa.start_page) && (page_ctr <= sa.end_page) {
				_, err := fout.Write([]byte(page))
				if err != nil {
					os.Exit(7)
				}
			}
			page_ctr++
		}
	}
	
	if page_ctr < sa.start_page {
		fmt.Fprintf(os.Stderr,
			"%s: start_page (%d) greater than total pages (%d)," +
			" no output written\n", progname, sa.start_page, page_ctr)
	} else if page_ctr < sa.end_page {
		fmt.Fprintf(os.Stderr,"%s: end_page (%d) greater than total pages (%d)," +
		" less output than expected\n", progname, sa.end_page, page_ctr)
	}
}