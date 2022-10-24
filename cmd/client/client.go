package main

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"internal/common"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Message struct {
	To      string
	From    string
	Content string
	Error   bool
}

func setup_connection(username string, ip string, port uint16) net.Conn {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))

	if err != nil {
		log.Fatal(err)
	}

	// Send size of username first
	err = binary.Write(conn, binary.BigEndian, uint8(len(username)))
	if err != nil {
		conn.Close()
		log.Fatal(err)
	}

	_, err = conn.Write([]byte(username))
	// _, err = fmt.Fprintf(conn, username)
	if err != nil {
		conn.Close()
		log.Fatal(err)
	}

	return conn
}

func send_message(conn net.Conn, to string, message string) {
	enc := gob.NewEncoder(conn)
	err := enc.Encode(Message{to, "", message, false})
	if err != nil {
		log.Fatal(err)
	}
}

func receive_error(conn net.Conn) {
	var error_length uint16
	err := binary.Read(conn, binary.BigEndian, &error_length)

	if err != nil {
		fmt.Println(err)
		return
	}

	raw_data := make([]byte, error_length)
	_, err = conn.Read(raw_data)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Fprint(common.ColorOutput, common.ErrorColor(string(raw_data)))
}

func receive_messages(conn net.Conn) {
	for {
		dec := gob.NewDecoder(conn)
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			log.Fatal(err)
		}

		if msg.Error {
			fmt.Fprint(common.ColorOutput, common.ErrorColor(string(msg.Content)))
			continue
		}

		fmt.Fprintf(common.ColorOutput, "%s: %s\n", common.NameColor(msg.From), common.MessageColor((msg.Content)))
	}
}

func handle_send_cmd(raw_cmd string, conn net.Conn) {
	full_tokens := strings.Split(raw_cmd, " ")
	args := full_tokens[1:]

	if len(args) < 2 {
		fmt.Println("Type 'send <username> <message>' to send a message")
		return
	}

	to := args[0]
	message := strings.Join(args[1:], " ")

	if len(to) > common.MAX_USERNAME_LENGTH {
		fmt.Fprint(
			common.ColorOutput,
			common.ErrorColor(
				fmt.Sprintf("Recipient username cannot be longer than %d characters\n", common.MAX_USERNAME_LENGTH)))
		return
	}

	if len(message) > common.MAX_MESSAGE_LENGTH {
		fmt.Fprint(
			common.ColorOutput,
			common.ErrorColor(
				fmt.Sprintf("Message cannot be longer than %d characters\n", common.MAX_MESSAGE_LENGTH)))
		return
	}

	go send_message(conn, to, message)
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Need to supply arguments: server ip, port, and username")
		return
	}

	server_ip := os.Args[1]
	username := os.Args[3]
	server_port, err := strconv.Atoi(os.Args[2])

	if err != nil {
		log.Fatal(err)
		panic(err)
	}

	if len(username) > common.MAX_USERNAME_LENGTH {
		fmt.Fprint(common.ColorOutput, common.ErrorColor(fmt.Sprintf("Username cannot be more than %d characters\n", common.MAX_USERNAME_LENGTH)))
		return
	}

	conn := setup_connection(username, server_ip, uint16(server_port))
	defer conn.Close()

	fmt.Fprintf(common.ColorOutput, "Connected with username %s\n", common.NameColor(username))

	go receive_messages(conn)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		raw_cmd := scanner.Text()

		if raw_cmd == "exit" {
			os.Exit(0)
		} else if strings.HasPrefix(raw_cmd, "send") {
			handle_send_cmd(raw_cmd, conn)
		} else {
			fmt.Println("Unrecognized command. Type 'send <username> <message>' or 'exit'")
		}
	}
}
