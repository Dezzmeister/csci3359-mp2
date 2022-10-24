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
)

type Message struct {
	To      string
	From    string
	Content string
	Error   bool
}

type Connection struct {
	username string
	conn     net.Conn
}

/*
Accept a connection request and return a struct identifying the connecting user.
When a new user connects, they send their username prefixed by a header indicating the length
of the username. The username is rejected if the length is too high because we don't
want malicious clients causing the server to allocate extra memory.
*/
func receive_connection(ln net.Listener) (Connection, error) {
	conn, err := ln.Accept()

	if err != nil {
		return Connection{}, err
	}

	// Size of username in bytes
	var username_size uint8
	err = binary.Read(conn, binary.BigEndian, &username_size)

	if err != nil {
		conn.Close()
		return Connection{}, err
	}

	// Reject long usernames. Errors are not sent to the client because we assume the client is malicious. Our
	// client does not allow you to send a long username
	if username_size > uint8(common.MAX_USERNAME_LENGTH) {
		conn.Close()
		return Connection{}, fmt.Errorf("username was too long: max %d characters", common.MAX_USERNAME_LENGTH)
	}

	raw_data := make([]byte, username_size)
	_, err = conn.Read(raw_data)

	if err != nil {
		conn.Close()
		return Connection{}, err
	}

	username := string(raw_data)

	fmt.Fprintf(common.ColorOutput, "User %s connected\n", common.NameColor(username))
	return Connection{username, conn}, nil
}

func receive_messages(connections map[string]Connection, conn Connection, mq chan<- Message) {
	for {
		dec := gob.NewDecoder(conn.conn)
		var msg Message
		err := dec.Decode(&msg)
		if err != nil {
			log.Fatal(err)
		}
		to_size := uint16(len(msg.To))
		msg_size := uint16(len(msg.Content))

		/*var opcode uint16

		err := binary.Read(conn.conn, binary.BigEndian, &opcode)

		if err != nil {
			break
		}

		header := make([]uint16, 2)
		err = binary.Read(conn.conn, binary.BigEndian, header)

		if err != nil {
			break
		} */

		/*to_size, msg_size := header[0], header[1]
		total_size := to_size + msg_size */

		if to_size > uint16(common.MAX_USERNAME_LENGTH) {
			fmt.Fprintf(
				common.ColorOutput,
				"User %s tried to send a message to a username of length %d. Max is %d\n",
				common.NameColor(conn.username),
				to_size,
				common.MAX_USERNAME_LENGTH)
			break
		}

		if msg_size > uint16(common.MAX_MESSAGE_LENGTH) {
			fmt.Fprintf(
				common.ColorOutput,
				"User %s tried to send a message of length %d. Max is %d\n",
				common.NameColor(conn.username),
				msg_size,
				common.MAX_MESSAGE_LENGTH)
			break
		}

		/*raw_data := make([]byte, total_size)
		_, err = conn.conn.Read(raw_data)
		if err != nil {
			break
		}

		to_username := string(raw_data[0:to_size])
		message := string(raw_data[to_size:total_size]) */

		msg.From = conn.username
		mq <- msg
		// mq <- Message{to_username, conn.username, message}
		//fmt.Fprintf(common.ColorOutput, "%s to %s: %s\n", common.NameColor(conn.username), common.NameColor(to_username), common.MessageColor(message))
		fmt.Fprintf(common.ColorOutput, "%s to %s: %s\n", common.NameColor(conn.username), common.NameColor(msg.To), common.MessageColor(msg.Content))
	}

	conn.conn.Close()
	delete(connections, conn.username)
	fmt.Fprintf(common.ColorOutput, "User %s disconnected or kicked\n", common.NameColor(conn.username))

}

func send_error(conn net.Conn, error_msg string) {
	msg := Message{"", "", error_msg, true}
	enc := gob.NewEncoder(conn)
	err := enc.Encode(msg)
	if err != nil {
		log.Fatal(err)
	}

	/*header := []uint16{common.ERROR_CODE, uint16(len(error_msg))}
	err := binary.Write(conn, binary.BigEndian, header)

	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = fmt.Fprintf(conn, error_msg)

	if err != nil {
		fmt.Println(err)
	} */
}

func process_message_queue(connections map[string]Connection, mq <-chan Message) {
	for {
		msg, ok := <-mq

		if !ok {
			return
		}

		to, ok := connections[msg.To]

		if !ok {
			fmt.Fprintf(common.ColorOutput, "User %s does not exist\n", common.NameColor(msg.To))

			from, ok := connections[msg.From]

			// Obscure edge case in which a sender disconnects immediately after sending a message, but before the message
			// is delivered. `connections` is shared among threads so another thread could delete a sender
			// before this thread has a chance to process the message.dec
			if !ok {
				fmt.Fprintf(common.ColorOutput, "Sender %s does not exist either. Dropping message\n", common.NameColor(msg.From))
				continue
			}

			send_error(from.conn, fmt.Sprintf("'%s' is not connected\n", msg.To))
			continue
		}

		fmt.Println("this should never print")

		header := []uint16{common.MESSAGE_CODE, uint16(len(msg.From)), uint16(len(msg.Content))}
		err := binary.Write(to.conn, binary.BigEndian, header)

		if err != nil {
			fmt.Println(err)
			continue
		}

		_, err = fmt.Fprintf(to.conn, "%s%s", msg.From, msg.Content)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
}

func listen_for_connections(ln net.Listener, connections map[string]Connection, mq chan<- Message) {
	for {
		conn, err := receive_connection(ln)

		if err != nil {
			fmt.Println(err)
			continue
		}

		_, ok := connections[conn.username]

		if ok {
			send_error(conn.conn, "Username is taken\n")
			conn.conn.Close()
			continue
		}

		connections[conn.username] = conn
		go receive_messages(connections, conn, mq)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Need to supply listening port for incoming connections")
		return
	}

	listen_port, err := strconv.Atoi(os.Args[1])

	if err != nil {
		panic(err)
	}

	connections := make(map[string]Connection)
	message_queue := make(chan Message, 256)
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", listen_port))

	if err != nil {
		panic(err)
	}

	go process_message_queue(connections, message_queue)
	fmt.Printf("Listening for connections\n")

	go listen_for_connections(ln, connections, message_queue)

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		if scanner.Text() == "exit" {
			os.Exit(0)
		} else {
			fmt.Println("Unrecognized command. Type 'exit' to quit")
		}
	}
}
