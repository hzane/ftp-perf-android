package main

import (
    "fmt"
    "os"
    "os/signal"
    "time"

    "github.com/sparrc/go-ping"
)

func mainPing() {
    pinger, err := ping.NewPinger(config.ping.host)
    if err != nil {
        fmt.Printf("ERROR: %s\n", err.Error())
        return
    }

    // listen for ctrl-C signal
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt)
    go func() {
        for range c {
            pinger.Stop()
        }
    }()

    pinger.OnRecv = func(pkt *ping.Packet) {
        fmt.Printf("goping-rtt %d\ttime=%v %d bytes from %s: icmp_seq=%d ttl=%v\n",
            pkt.Rtt.Nanoseconds()/int64(time.Microsecond), pkt.Rtt, pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Ttl)
    }
    pinger.OnFinish = func(stats *ping.Statistics) {
        fmt.Printf("goping-transmitted-packets %d\trecved-packets %d\tloss %v%%\n",
            stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
        fmt.Printf("goping-round-trip min %v\tavg %v\tmax %v\tstddev %v\n",
            stats.MinRtt.Nanoseconds()/int64(time.Microsecond),
            stats.AvgRtt.Nanoseconds()/int64(time.Microsecond),
            stats.MaxRtt.Nanoseconds()/int64(time.Microsecond),
            stats.StdDevRtt.Nanoseconds()/int64(time.Microsecond))
    }
    pinger.Size = config.ping.size
    pinger.Count = config.ping.count
    pinger.Interval = config.ping.interval
    pinger.Timeout = config.ping.timeout
    pinger.SetPrivileged(config.ping.privileged)

    fmt.Printf("ping %s (%s):\n", pinger.Addr(), pinger.IPAddr())
    pinger.Run()
}
