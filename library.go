package main

import (
	"fmt"
	"bufio"
	"os"
	// mysql connector
	_ "github.com/go-sql-driver/mysql"
	sqlx "github.com/jmoiron/sqlx"
)

const (
	User     = "root"
	Password = "123456"
	DBName   = "ass3"
)

type account struct {
	username string
	password string
	is_admin int
	suspend int
}

type Library struct {
	db *sqlx.DB
	acc account
	stat bool
}
// global varibles
var cntBook int
var month = [12]int {
	31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31,
}

// end global varibles

func execsqls(db *sqlx.DB, sqls []string){
	for _, sql := range sqls{
		_, err := db.Exec(sql)
		if err != nil{              
			panic(err)
		}
	}
}

func (lib *Library) ConnectDB() {
	db, err := sqlx.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s", User, Password, DBName))
	if err != nil {
		panic(err)
	}
	lib.db = db
}

// CreateTables created the tables in MySQL
func (lib *Library) CreateTables() error {
	execsqls(lib.db, []string{
		"create table account (username char(16) primary key, password char(16), is_admin int, suspend int)",
		"create table book (id int primary key, title char(64), author char(64), ISBN char(16), remove int)", 
		`create table borrow (
			id int,
			username char(16),
			day int,
			extend int,
			is_returned int,
			foreign key (id) references book(id),
			foreign key (username) references account(username))
		`,
		"create table remove (id int, expl char(128))",
	})
	return nil
}

func NewLibrary() *Library {
	db, err := sqlx.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/", User, Password))
	if err != nil {
		panic(err)
	}
	execsqls(db, []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS %s", DBName),
		fmt.Sprintf("CREATE DATABASE %s", DBName),
	})

	lib := &Library{
		db:  nil,
		acc: account{
			username: "",
			password: "",
			is_admin: 0,
		},
		stat: false,
	}
	lib.ConnectDB()
	lib.CreateTables()
	return lib
}

// utility functions ------------------------
func dateToInt(date string) int{
	l := len(date)
	var num = [3]int {0, 0, 0,}
	cnt := 0
	for i := 0; i < l; i++ {
		if date[i] >= '0' && date[i] <= '9' {
			num[cnt] = num[cnt] * 10 + int(date[i]) - '0'
		}else{
			cnt++
		}
	}
	num[0] %= 100
	res := 0
	for i := 0; i < num[0]; i++ {
		if i % 4 == 0 {
			res += 366
		}else {
			res += 365
		}
	}
	for i := 0; i < num[1] - 1; i++ {
		res += month[i]
		if i == 1 && num[0] % 4 == 0 {
			res++
		}
	}
	res += num[2]
	return res
}
func intToDate(num int) string{
	day := num
	var year, mon int
	for year = 0;; year++ {
		s := 365
		if year % 4 == 0 {
			s++
		}
		if day - s <= 0 {
			break
		}
		day -= s
	}
	is_leap := year % 4 == 0
	for mon = 0;; mon++ {
		s := month[mon]
		if is_leap && mon == 1 {
			s++
		}
		if day - s <= 0 {
			break
		}
		day -= s
	}
	return fmt.Sprintf("2%03d.%02d.%02d", year, mon + 1, day)
}
func (lib *Library) checkRoot() bool{
	root := lib.acc.is_admin
	if root == 0 {
		fmt.Println("E: Cannot execute it. Are you admin?")
		return false
	}
	return true
}
// root's priority
func (lib *Library) AddBook(title, author, ISBN string){
	db := lib.db
	if lib.checkRoot() {
		cntBook++
		execsqls(db, []string{
			fmt.Sprintf("insert into book values(%d, '%s', '%s', '%s', 0)", cntBook, title, author, ISBN),
		})
		fmt.Printf("Add book id = %d, title = '%s', author = '%s', ISBN : '%s'\n", cntBook, title, author, ISBN)
	}
}
func (lib *Library) AddAccount(username, password string){
	db := lib.db
	if lib.checkRoot() {
		row, err := db.Query(fmt.Sprintf("select * from account where username = '%s'", username))
		if err != nil{
			panic(err)
		}
		cnt := 0
		for row.Next(){
			cnt++
		}
		if cnt > 0{
			fmt.Printf("E: User '%s' already exists!\n", username)
			return
		}
		execsqls(db, []string{
			fmt.Sprintf("insert into account values('%s', '%s', 0, 0)", username, password),
			})
		fmt.Printf("Student account '%s' has been added.\n", username)
	}
}
func (lib *Library) CheckDDL(id int, usr string){
	db := lib.db
	row, err := db.Query(fmt.Sprintf("select day from borrow where id = %d and username = '%s' and is_returned = 0", id, usr))
	if err != nil {
		panic(err)
	}
	cnt := 0
	var date int
	for row.Next(){
		row.Scan(&date)
		cnt++
	}
	if usr == lib.acc.username || lib.checkRoot(){
		if cnt == 0{
			fmt.Println("E: No such record.")
			return
		}
		fmt.Printf("The deadline is %s.\n", intToDate(date + 90))
	}
}//check the deadline of returning a borrowed book
func (lib *Library) RemoveBook(id int, expl string){
	db := lib.db
	if lib.checkRoot() {
		row, err := db.Query(fmt.Sprintf("select title from book where id = %d", id))
		if err != nil {
			panic(err)
		}
		cnt := 0
		var title string
		for row.Next() {
			row.Scan(&title)
			cnt++
		}
		if cnt == 0 {
			fmt.Println("E: No such book.")
		}
		db.Exec(fmt.Sprintf("update book set remove = 1 where id = %d", id))
		db.Exec(fmt.Sprintf("insert into remove values(%d, '%s')", id, expl))
		fmt.Printf("Remove book id = %d, title = '%s'.\nExplanation: %s.", id, title, expl)
	}
}//remove a book from the library with explanation (e.g. book is lost)
func (lib *Library) ExtendDDL(id int, username string){
	db := lib.db
	if username == lib.acc.username || lib.checkRoot() {
		row, err := db.Query(fmt.Sprintf("select extend from borrow where id = %d and username = '%s' and is_returned = 0", id, lib.acc.username))
		if err != nil {
			panic(err)
		}
		cnt := 0
		var extend int
		for row.Next() {
			cnt++
			row.Scan(&extend)
		}
		if cnt == 0 {
			fmt.Println("E: No such book. Have the user ever borrow it?")
		}
		if extend == 3 {
			fmt.Println("E: The deadline has been extened 3 times. Please contact the admin for more information.")
		}else {
			extend++
			db.Exec(fmt.Sprintf("update borrow set extend = %d where id = %d and username = '%s' and is_returned = 0", extend, id, username))
			fmt.Printf("Extend the deadline of book id = %d for 90 days.\n", id)
		}
	}
}//extend the deadline of returning a book, at most 3 times (i.e. refuse to extend if the deadline has been extended for 3 times)
func (lib *Library) QueryHistory(username string){
	db := lib.db
	if username == lib.acc.username || lib.checkRoot() {
		rows, err := db.Query(fmt.Sprintf("select book.id, title, day from book, borrow where username = '%s' and book.id = borrow.id order by day", username))
		if err != nil{
			panic(err)
		}
		fmt.Println("---------------------")
		cnt := 0
		var title string
		var dateInt, id int
		for rows.Next() {
			cnt++
			rows.Scan(&id, &title, &dateInt)
			fmt.Printf("%3d. id = %d, title = '%s', date = %s\n", cnt, id, title, intToDate(dateInt))
		}
		fmt.Println("---------------------")
		fmt.Printf("Total %d records\n", cnt)
	}
}// query the borrow history of a student account
func (lib *Library) QueryNotReturn(username string){
	db := lib.db
	if username == lib.acc.username || lib.checkRoot() {
		rows, err := db.Query(fmt.Sprintf("select book.id, title from book, borrow where borrow.id = book.id and borrow.username = '%s' and borrow.is_returned = 0", username))
		if err != nil {
			panic(err)
		}

		fmt.Println("---------------------")
		cnt := 0
		var id int
		var title string
		for rows.Next() {
			cnt++
			rows.Scan(&id, &title)
			fmt.Printf("%3d. id = %d, title = '%s'\n", cnt, id, title)
		}
		fmt.Println("---------------------")
		fmt.Printf("Total %d records\n", cnt)
	}
}//query the books a student has borrowed and not returned yet
func (lib *Library) QueryOverdue(username, date string){
	db := lib.db
	if username == lib.acc.username || lib.checkRoot() {
		ndate := dateToInt(date)
		rows, err := db.Query(fmt.Sprintf(
			`select book.id, book.title, borrow.day, borrow.extend from book, borrow 
			where book.id = borrow.id and is_returned = 0 and username = '%s' and borrow.day + 90 * (extend + 1) < %d`, username, ndate))
		if err != nil {
			panic(err)
		}

		fmt.Println("---------------------")
		cnt := 0
		var id, dateInt, extend int
		var title string
		for rows.Next() {
			cnt++
			rows.Scan(&id, &title, &dateInt, &extend)
			duedate := intToDate(dateInt + (extend + 1) * 90)
			fmt.Printf("%3d. id = %d, title = '%s', duedate: %s\n", cnt, id, title, duedate)
		}
		fmt.Println("---------------------")
		fmt.Printf("Total %d records\n", cnt)
	}
}//check if a student has any overdue books that needs to be returned
func (lib *Library) SuspendAccount(date string){
	db := lib.db
	if lib.checkRoot() {
		rows, err := db.Query(fmt.Sprintf(
			`select username, count(*) as cnt from borrow
			where is_returned = 0 and day + 90 * (extend + 1) < %d
			group by username
			having cnt > 3`, dateToInt(date)))
		if err != nil {
			panic(err)
		}
		var cnt int
		var username string
		for rows.Next() {
			rows.Scan(&username, &cnt)
			db.Exec(fmt.Sprintf("update account set suspend = 1 where username = '%s'", username))
			fmt.Printf("Account '%s' has been suspended.\n", username)
		}
	}
}

// student's privilege
func (lib *Library) QueryMyHistory(){
	lib.QueryHistory(lib.acc.username)
}
func (lib *Library) QueryMyNotReturn(){
	lib.QueryNotReturn(lib.acc.username)
}
func (lib *Library) CheckMyDDL(id int){
	lib.CheckDDL(id, lib.acc.username)
}
func (lib *Library) ExtendMyDDL(id int){
	lib.ExtendDDL(id, lib.acc.username)
}
func (lib *Library) QueryMyOverdue(date string){
	lib.QueryOverdue(lib.acc.username, date)
}
func (lib *Library) QueryBook(ins string){
	db := lib.db
	rows, err := db.Query(fmt.Sprintf("select * from book where remove = 0 and %s and not exists (select * from borrow where is_returned = 0 and borrow.id = book.id)", ins))
	if err != nil {
		//panic(err)
		fmt.Println("E: Input invalid querying instruction.")
		return
	}
	var id int
	var title, author, ISBN string
	cnt := 0
	fmt.Println("---------------------")
	for rows.Next(){
		cnt++
		rows.Scan(&id, &title, &author, &ISBN)
		fmt.Printf("%3d. id = %d, title = '%s', author = '%s', ISBN = '%s'\n", cnt, id, title, author, ISBN)
	}
	fmt.Println("---------------------")
	fmt.Printf("Total %d records\n", cnt)
}
func (lib *Library) BorrowBook(id int, date string){
	db := lib.db
	if lib.acc.suspend == 1 {
		fmt.Println("E: Your account has been suspended. Please return books to eliminate the suspension.")
		return
	}
	row, err := db.Query(fmt.Sprintf(
		`select book.title from book
		 where book.id = %d and remove = 0 and not exists(
		 	select * from borrow
		 	where book.id = borrow.id and borrow.is_returned = 0
		 )`, id))
	if err != nil {
		panic(err)
	}
	cnt := 0
	var title string
	for row.Next() {
		cnt++
		row.Scan(&title)
	}
	if cnt == 0 {
		fmt.Println("E: No such book or it has been borrowed.")
		return
	}
	fmt.Printf("Borrow book id = %d, title = '%s' in %s by %s\n", id, title, intToDate(dateToInt(date)), lib.acc.username)
	_, err = db.Exec(fmt.Sprintf("insert into borrow values(%d, '%s', %d, 0, 0)", id, lib.acc.username, dateToInt(date)))
	if err != nil {
		panic(err)
	}
}//borrow a book from the library with a student account
func (lib *Library) ReturnBook(id int){
	db := lib.db
	usr := lib.acc.username
	row, err := db.Query(fmt.Sprintf("select * from borrow where id = %d and username = '%s' and is_returned = 0", id, usr))
	if err != nil {
		panic(err)
	}
	cnt := 0
	for row.Next() {
		cnt++
	}
	if cnt == 0 {
		fmt.Println("E: No such book. Have you borrowed it?")
		return
	}
	db.Exec(fmt.Sprintf(`
		update borrow
		set is_returned = 1
		where id = %d and username = '%s' and is_returned = 0
		`, id, usr))
	fmt.Printf("Return book %d.\n", id)
}//return a book to the library by a student account (make sure the student has borrowed the book)
func (lib *Library) CheckValid(date string){
	db := lib.db
	rows, err := db.Query(fmt.Sprintf(
		`
			select count(*) from borrow
			where is_returned = 0 and borrow.day + 90 * (extend + 1) < %d and username = '%s'
		`, dateToInt(date), lib.acc.username))
	if err != nil {
		panic(err)
	}
	var cnt int
	for rows.Next() {
		rows.Scan(&cnt)
	}
	if cnt <= 3 {
		db.Exec(fmt.Sprintf("update account set suspend = 0 where username = '%s'", lib.acc.username))
		fmt.Println("Your suspension has been cancelled.\n")
		lib.acc.suspend = 0
	}else {
		fmt.Println("You still have more than 3 books overdue. Please return them soon!")
	}
}


// end utility functions ------------------------

func eliminateSpace(s string) string{
	l := len(s)
	st := 0
	ed := l - 1
	for i := 0; i < l; i++{
		if s[i] != ' '{
			st = i
			break
		}
	}
	for i := ed; i > 0; i--{
		if s[i - 1] != ' '{
			ed = i
			break
		}
	}
	w := s[st:ed]
	return w
}


func (lib *Library) enroll(){
	var ins, username, password, rpassword string
	db := lib.db
	for ;; {
		fmt.Print("Please input your username, or input 'exit' to leave, space use is not allowed!\n> ")
		fmt.Scan(&ins)
		if ins == "exit" {
			fmt.Println("Creating account cancelled.")
			return
		}

		username = ins

		row, err := db.Query(fmt.Sprintf("select * from account where username = '%s'", username))
		if err != nil{
			panic(err)
		}

		cnt := 0
		for row.Next(){
			cnt++
		}
		if cnt > 0{
			fmt.Printf("E: User '%s' already exists!\n", username)
			continue
		}

		if len(username) > 16 {
			fmt.Println("E: Username is too long!")
			continue
		}

		fmt.Print("Please input your password, space use is not allowed!\n> ")
		fmt.Scan(&password)
		fmt.Print("Please confirm your password.\n> ")
		fmt.Scan(&rpassword)
		if password != rpassword {
			fmt.Println("E: Password doesn't match!")
			continue
		}

		fmt.Print("Please enter the password of the database to verify your identity.\n> ")
		var pwd string
		fmt.Scan(&pwd)
		if pwd == Password {
			fmt.Println("Authenticate successfully.")
		}else {
			fmt.Println("E: Invalid input.")
			continue
		}
		break
	}
	fmt.Printf("Admin account '%s' has been created successfully!\n", username)
	execsqls(db, []string{
		fmt.Sprintf("insert into account values('%s', '%s', 1, 0)", username, password), 
		})
}

func (lib *Library) login(){
	var username, password string
	var usr, pwd string
	var is_admin, suspend int
	db := lib.db
	for ;; {
		fmt.Print("Please enter your username, or input 'exit' to leave.\n> ")

		var ins string
		fmt.Scanln(&ins)
		if ins == "exit" {
			fmt.Println("Login cancelled.")
			return
		}

		username = ins

		row, err := db.Query(fmt.Sprintf("select * from account where username = '%s'", username))
		if err != nil{
			panic(err)
		}
		cnt := 0
		for row.Next() {
			cnt++
			row.Scan(&usr, &pwd, &is_admin, &suspend)
		}
		if cnt == 0 {
			fmt.Println("E: No such user!")
			continue
		}
		fmt.Print("Please enter your password.\n> ")
		fmt.Scan(&password)
		if pwd != password {
			fmt.Println("E: Wrong password!")
		}else{
			fmt.Println("Login successfully!")
			break
		}
	}

	lib.acc = account{
		username: username,
		password: pwd,
		is_admin: is_admin,
		suspend : suspend,
	}
	lib.stat = true
}

func (lib *Library) quit(){
	fmt.Printf("Bye! %s!\n", lib.acc.username)
	lib.acc = account{
		username: "",
		password: "",
		is_admin: 0,
		suspend : 0,
	}
	lib.stat = false
}

func submeun(lib *Library){
	var inputReader *bufio.Reader
	inputReader = bufio.NewReader(os.Stdin)
	root := lib.acc.is_admin
	var opt string
	for ;; {
		if root == 1 {
			fmt.Print("admin@library# ")
		}else{
			fmt.Print("student@library# ")
		}
		// str, err := inputReader.ReadString('\n')
		// if err != nil {
		// 	panic(err)
		// }
		fmt.Scan(&opt)
		if opt == "addbook" {
			var title, author, isbn string
			fmt.Scan(&title, &author, &isbn)
			lib.AddBook(title, author, isbn)
		}else if opt == "removebook" {
			var id int
			fmt.Scan(&id)
			str, err := inputReader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			lib.RemoveBook(id, str)
		}else if opt == "addaccount" {
			var usr, pwd string
			fmt.Scan(&usr, &pwd)
			lib.AddAccount(usr, pwd)
		}else if opt == "querybook" {
			str, err := inputReader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			lib.QueryBook(str)
		}else if opt == "borrowbook" {
			var id int
			var date string
			fmt.Scan(&id, &date)
			lib.BorrowBook(id, date)
		}else if opt == "queryhistory" {
			var usr string
			fmt.Scan(&usr)
			lib.QueryHistory(usr)
		}else if opt == "querymyhistory" {
			lib.QueryMyHistory()
		}else if opt == "querynotreturn" {
			var usr string
			fmt.Scan(&usr)
			lib.QueryNotReturn(usr)
		}else if opt == "querymynotreturn" {
			lib.QueryMyNotReturn()
		}else if opt == "checkddl" {
			var id int
			var usr string
			fmt.Scan(&usr, &id)
			lib.CheckDDL(id, usr)
		}else if opt == "checkmyddl" {
			var id int
			fmt.Scan(&id)
			lib.CheckMyDDL(id)
		}else if opt == "extendddl" {
			var id int
			var usr string
			fmt.Scan(&usr, &id)
			lib.ExtendDDL(id, usr)
		}else if opt == "extendmyddl" {
			var id int
			fmt.Scan(&id)
			lib.ExtendMyDDL(id)
		}else if opt == "queryoverdue" {
			var usr, date string
			fmt.Scan(&usr, &date)
			lib.QueryOverdue(usr, date)
		}else if opt == "querymyoverdue"{
			var date string
			fmt.Scan(&date)
			lib.QueryMyOverdue(date)
		}else if opt == "returnbook" {
			var id int
			fmt.Scan(&id)
			lib.ReturnBook(id)
		}else if opt == "suspend" {
			var date string
			fmt.Scan(&date)
			lib.SuspendAccount(date)
		}else if opt == "checkvalid" {
			var date string
			fmt.Scan(&date)
			lib.CheckValid(date)
		}else  if opt == "quit" || opt == "exit" || opt == "logout" {
			lib.quit()
			break
		}else if opt == "login" {
			fmt.Println("E: Please logout first!")
		}else{
			fmt.Println("E: Invalid input, please reinput or look up the guideline to know how to use.")
		}
	}
}

func mainmeun(lib *Library){
	fmt.Print("Welcome to the Library Management System!\n> ")

	var inputReader *bufio.Reader
	inputReader = bufio.NewReader(os.Stdin)
	for ;; {
		str, err := inputReader.ReadString('\n')
		if err != nil{
			panic(err)
		}

		w := eliminateSpace(str)
		if len(w) == 0 {
			fmt.Print("> ")
			continue
		}

		if w == "exit"{
			fmt.Println("Goodbye!")
			break
		}
		
		if w == "?" {
			fmt.Println("Input 'login' to sign in, 'enroll' to sign up an admin account or 'exit' to exit")
		}else if w == "login"{
			lib.login()
			if lib.stat {
				submeun(lib)
			}
		}else if w == "enroll"{
			lib.enroll()
		}else if w == "quit" || w == "logout" {
			fmt.Println("E: Please sign in first!")
		}else{
			fmt.Println("E: Invalid input, input ? to check out more information")
		}

		fmt.Print("> ")
	}
}

func main() {
	lib := NewLibrary()
	mainmeun(lib)
}