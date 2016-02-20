# p (short for "punch")
Punch tool in Go using SQLite as storage

The reason I call it just "p" is because I actually
call it quite often during work (to punch in, punch out, etc).

Calling it using only one letter is also something that
impressed me from the 't' tool. Google it... :)
No, just kidding, it is "todo.txt": http://todotxt.com/

# Origin
This was initially a spin-off to a similar program
I made in JavaScript: https://github.com/jramb/punch

Working with JavaScript makes you want to dip your fingers
into hot lava after a while. Also for testing Go I started
with a rewrite. After a while I also found out that SQLite is a
much better back-end to store time data.

Mostly because SQL rocks and SQLite is a really good file format.

Punch was also inspired by "todo.txt", which also is recommended
to be installed as a single character command: `t`: http://todotxt.com/

Of course I abandoned the idea of having my database in an org-flat-file
structure, event if that was very convenient for "database maintenance".

