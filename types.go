package main

type User struct {
	Id       int    `db:"id"`
	Username string `db:"username"`
}

type Transaction struct {
}

func loadUser(id int) (u User, err error) {
	err = pg.Get(&u, `
SELECT id, username
FROM account
WHERE id = $1
    `, id)
	return
}

func createUser(id int, username string) (u User, err error) {
	_, err = pg.Exec(`
INSERT INTO account (id, username)
VALUES ($1, $2)
ON CONFLICT (id) SET username = $2
    `, id, username)
	if err != nil {
		return
	}

	u = User{id, username}
	return
}
