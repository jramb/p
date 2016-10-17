# p (short for "punch")
Punch tool in Go using SQLite as storage

The reason I call it just "p" is because I actually
call it quite often during work (to punch in, punch out, etc).

Calling it using only one letter is also something that
impressed me from the elegance of the 't' todo.txt tool:
http://todotxt.com/

Be warned, it is a command line tool, currently no fancy web interface or mobile
app exists. Maybe in the future, but it works good as it is now.

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

# Purpose
This is mainly MY tool for my own needs, but I think you might have
use for it too. If you have any good ideas, please let me know.


# Usage
The program itself contains a good deal of documentation. You can access it using

    p help

or get more detailed help for specific command like:

    p help show


## Preparation
Before you can use punch properly, just a quick setup is necessary.

Decide where your database file should reside. It is not very big and grows
quite slowly, mine is about 120k, containing several years of data.

For a quick start, lets put it into your current directory and call it `timetracker.org.db`.
Later you can simply move it to a better, safer place. This single file contains all
your punch data.

Place the `p` (or `p.exe` on Windows) binary somewhere in your path.

Create a config file (you need this) in either the current directory (`.`), in `~/.config`,
or on Windows in your home directory (%USERPROFILE%).

The config file is named `punch.toml`. You can start with this contents (customize at will!):

    clockfile = "timetracker.org.db" # current directory
    #OR FOR EXAMPLE: clockfile = "/home/jramb/.time/timetracker.org.db" 
    debug = false

    [show]
    rounding = "30m"        # default is "1m"
    bias = "5m"             # default is "0m"
    display-rounding = true # default is false

Alternatively use the YAML format: `.config/punch.yaml`:

    clockfile: timetracker.org.db
    debug: false
    show:
      rounding: 30m
      bias: 5m
      display-rounding: true


The DB file needs to be created before use, call this:

    p initialize

Don't worry, you can call this even after the database is created, no harm.
But you need to run it once.

Now add att least one *header* to your database. A header could be a project or an assignment
that you want to track time for. If you want to switch between several  headers, no problem,
but at least one needs to be created before you can _punch in_.

    p head add @test Testing the timetracker

Now the `@test` is the handler, which makes it easier for you to refer to this header. The rest
(`Testing the timetracker`) is just a title for the handler.
Just for fun, add another header:

    p head add @dev Develop something cool

You can have many headers or only one, but to count your time in some bucket, you need at least
one header.

That all was the hard part and I hope it was not too hard... :)

## Usage
### Punch in and out
When you start an activity, you "punch in" like this:

    p in @test

That's all, time is now ticking. Do testing. Or, since you have nothing to test (maybe),
at any time switch to another task by punching into that:

    p in @dev

If you want to check what is currently running:

    p ru

(that is short for "running", I am lazy). `p ru` will print nothing if you are not punched
in, otherwise it will show the time spend in that running entry only.

Now assume you are done, punch out:

    p out

This is the basics to register time entries.

Worth mentioning is the time modification function. Assume you forgot to punch in
one day. After say one hour and 20 minutes you realise that and want to punch in
afterwards. The `-m` modifier allows you to turn back the effective time accordingly,
in our example you punch in using this:

    p in @dev -m 1h20m

This works on both punching in the first time during this day or when switching
to another task. It also works when punching out, just the same way.

You can also specify a negative modifier, such as `-m -15m`, which would move
the effective time forward. Now you can punch out earlier and still registering
the (estimated) additional time you will spend on that task. For example if your
new Windows laptop insists on updating every day, which takes 10 minutes, punch
out like a boss:

    p out -m -10m

(since the modifier is negative, your punch out time is registered as 10m(inutes)
from now.)


### Simple reporting
Now at the end of the month (or week), you would like to look back at your life and
see what you have done. Well, `punch` can not help you with that. But it can show
you how much time you spent on the different tasks!

Issue this:

    p show sum

The output will be something like this:

    Headers: 2016-10-10 -- 2016-10-16
                    20:00  +0:07  Testing another one @foo
                     1:30  -0:15  Testing another one @bar
                     0:30  +0:01  Testing this @test
         Total:     22:00  -0:07


This shows you a summary of every header during the current week (default)
To see last week:

    p show sum week-1

To see the current month:

    p show sum month

Or to look at the previous month:

    p show sum month-1

You get the idea. Instead of `sum` summary you can get the more detailed daily summary
by using `show days`:

    p show days

Again, all the period indicators work the same as with `show sum`. If you don't remember
all the details, `p help show` is your friend.

One short note about the `rounding` stuff. Punch registers time entries exact (well, rounded to the minute).
When using `show` you can apply automatic rounding to hours or halv hours, since most often the minute
details are not interesting. Simple rounding will round the durations spent on a task
up or down, as you would expect it (but note that the summarization level can give different
totals depending on the period).

When rounding to 30m (half an hour):

* 4:14 becomes 4:00
* 2:16 becomes 2:30
* etc.

Now some consider rounding down a no-go... My plummer does that, when he works 5 minutes
that is rounded up to 0:30 and I do not complain!

If you want to achieve that, use the `bias` setup (either in your `.punch.yaml` or in the command line).
For example, the plummer will use this command:

    p show days --rounding 30m --bias 15m -r

(The `-r` flag also displays the rounded value). Technically, the bias is
simply added to the duration before rounding, which in above case results in
that durations are always rounded up to the nearest rounding factor.  A bias of
`0m` is fair in the long run, everything in between is a negotiation. Punch will not
use a bias larger than half the rounding.


### TODO handling
Punch contains a very simple TODO handler. It is not at all meant to be comprehensiv,
but the little advantage of it is that TODOs are/can be context sensitive and can be
applied to the currently checked in header only.

    p help todo

### LOG handling
Punch also contains an extremly simple LOG mechanism.

    p help log

# Conclusion
I use this tool myself a lot, after having tried several other tools. I guess it fits my own
needs best, but maybe you like it too.

