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
		panic(err)
	}

	// Send size of username first
	err = binary.Write(conn, binary.BigEndian, uint8(len(username)))
	if err != nil {
		conn.Close()
		panic(err)
	}

	_, err = fmt.Fprintf(conn, username)
	if err != nil {
		conn.Close()
		panic(err)
	}

	return conn
}

func send_message(conn net.Conn, to string, message string) {
	enc := gob.NewEncoder(conn)
	err := enc.Encode(Message{to, "", message, false})
	if err != nil {
		// can Fsprintf what you want here.
		log.Fatal(err)
	}
	/*header := []uint16{common.MESSAGE_CODE, uint16(len(to)), uint16(len(message))}
	err := binary.Write(conn, binary.BigEndian, header)

	if err != nil {
		fmt.Println(err)
		return
	} */

	/*_, err = fmt.Fprintf(conn, "%s%s", to, message)

	if err != nil {
		fmt.Println(err)
		return
	} */
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

	fmt.Fprintf(common.ColorOutput, common.ErrorColor(string(raw_data)))
}

func receive_messages(conn net.Conn) {
	for {
		dec := gob.NewDecoder(conn)
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if msg.Error {
			fmt.Fprint(common.ColorOutput, common.ErrorColor(string(msg.Content)))
			continue
		}

		/*var msg_format uint16
		err := binary.Read(conn, binary.BigEndian, &msg_format)

		if err != nil {
			panic(err)
		}

		if msg_format == common.ERROR_CODE {
			receive_error(conn)
			continue
		}

		header := make([]uint16, 2)
		err = binary.Read(conn, binary.BigEndian, header)

		if err != nil {
			panic(err)
		}

		from_size, msg_size := header[0], header[1]

		total_size := from_size + msg_size

		raw_data := make([]byte, total_size)
		_, err = conn.Read(raw_data)

		if err != nil {
			panic(err)
		} *

		from := string(raw_data[0:from_size])
		message := string(raw_data[from_size:total_size]) */

		// fmt.Fprintf(common.ColorOutput, "%s: %s\n", common.NameColor(from), common.MessageColor((message)))
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
		fmt.Fprintf(
			common.ColorOutput,
			common.ErrorColor(
				fmt.Sprintf("Recipient username cannot be longer than %d characters\n", common.MAX_USERNAME_LENGTH)))
		return
	}

	if len(message) > common.MAX_MESSAGE_LENGTH {
		fmt.Fprintf(
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
		panic(err)
	}

	if len(username) > common.MAX_USERNAME_LENGTH {
		fmt.Fprintf(common.ColorOutput, common.ErrorColor(fmt.Sprintf("Username cannot be more than %d characters\n", common.MAX_USERNAME_LENGTH)))
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
