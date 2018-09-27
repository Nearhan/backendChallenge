/*jslint node: true */
"use strict";

// Imports
var dgram = require("dgram");
var crypto = require("crypto");
var assert = require("assert");

// Globals
var PORT = 6789;
var HOST = "127.0.0.1";
var NUM_SRC_SKTS = 16;
var PKT_MAX_SIZE = 512;
var PKT_HDR_SIZE = 12;
var PKT_MAX_DATA_SIZE = (PKT_MAX_SIZE - PKT_HDR_SIZE);
var MAX_PAYLOAD_SIZE = (1024 * 1024);

Array.prototype.randomElement = function () {
    return this[Math.floor(Math.random() * this.length)];
}

// Generate random sequence of (offset, len) tuples that range input value top
// eg:
// input 10, 3
// output: [(3,1), (0,3), (7,2), (9,1), (4, 3)]
//
function genRandom(top, max) {
    // generate a list of ranges, decompose iteratively
    var ranges = [[0, top]];
    var node;
    var nnode;
    var offset;
    var noffset;
    var maxOffset = 0;
    var outSeq = [];

    while (ranges.length > 0) {
        // split on random range offset
        offset = Math.floor(Math.random() * ranges.length);

        // grab the target node
        node = ranges[offset];

        // split a random node
        noffset = Math.floor(Math.random() * node[1]);
        nnode = [node[0] + noffset, node[1] - noffset];

        // limit size to max
        if (nnode[1] > max) {
            // insert new node
            ranges.splice(offset + 1, 0, [nnode[0] + max, nnode[1] - max]);
            // fix up size of new node
            nnode[1] = max;
        }

        // emit the node
        outSeq.push(nnode);

        if (nnode[0] > maxOffset) {
            maxOffset = nnode[0];
        }

        // split or remove old node
        if (noffset === 0) {
            // remove original node
            ranges.splice(offset, 1);

        } else {
            // must trim node
            node[1] = noffset;
        }
    }

    //console.log("before return of gen");
    //console.log(outSeq.length);
    //console.log(outSeq[outSeq.lenth -1 ])
    //console.log(maxOffset)


    return {
        "sequence": outSeq,
        "maxOffset": maxOffset
    };
}

// Emit UDP packet to HOST:PORT
// Packet Structure
// Flags   uint16_t  BE  (high bit indicates EOF)
// DataSz  uint16_t  BE
// Offset  uint32_t  BE
// TransID uint32_t  BE
// Data    N DataSz Bytes
//
function emitPacket(skts, trans_id, data, offset, isLast, cb) {
    assert(data.length <= PKT_MAX_DATA_SIZE);

    // Determine pkt size
    var pktSize = PKT_HDR_SIZE + data.length;

    // Create packet buffer
    var pkt = new Buffer(pktSize);

    // Write the header
    var flags = 0;
    if (isLast) {
        //console.log("inside is late", data.length)
        flags = 0x8000;
    }


    pkt.writeUInt16BE(flags, 0);
    pkt.writeUInt16BE(data.length, 2);
    pkt.writeUInt32BE(offset, 4);
    pkt.writeUInt32BE(trans_id, 8);

    // Append the payload
    data.copy(pkt, PKT_HDR_SIZE);

    // Put the data on the wire, random skt.
    skts.randomElement().send(pkt, 0, pkt.length, PORT, HOST, cb);
}

// Emit buffer payload randomly via UDP skt
//
function emitRandom(skts, trans_id, payload, rObj, cb) {
    var cnt = 0;
    var l = rObj.sequence;
    var maxOffset = rObj.maxOffset;

    function sendOne() {
        var node = l[cnt];
        var data = payload.slice(node[0], node[1] + node[0]);
        //console.log(node)
        //console.log(data)
        //var waitTill = new Date(new Date().getTime() + 1 * 1000);
        //while(waitTill > new Date()){}
        

        var isLast = (node[0] === maxOffset);
        emitPacket(skts, trans_id, data, node[0], isLast, function (err) {
            if (err) {
                console.log("Error emitting: " + err);
                cb(err);
            }

            cnt += 1;
            if (cnt >= l.length) {
                cb(null, cnt);
            } else {
                sendOne();
            }
        });
    }

    sendOne();
}

// Emit one payload from [1,MAX_PAYLOAD_SIZE] randomly
//
function emitPayload(skts, trans_id, cb) {
    // create random data payload between 1 byte and MAX_PAYLOAD_SIZE inclusive
    var dataSz = Math.floor((Math.random() * MAX_PAYLOAD_SIZE) + 1);
    var pload = crypto.randomBytes(dataSz);
    var hash = crypto.createHash("sha256").update(pload);
    var rObj = genRandom(pload.length, PKT_MAX_DATA_SIZE);

    emitRandom(skts, trans_id, pload, rObj, function (err, cnt) {
        console.log("Emitted message #" + trans_id + " of size:" + dataSz + " nPkts:" + cnt + " sha256:" + hash.digest("hex"));
        cb(err);
    });
}

// Main function
//
function main() {
    var skt;
    var skts = [];
    var trans_id = 0;
    var cnt = 10;
    var i;

    // Emit randomly from NUM_SRC_SKTS ports.
    // This allows operating systems such as 'linux'
    // to distribute packets across different processes
    // listening on the same port via a hash of the tuple
    // (src_addr, src_port, dst_addr, dst_port)
    // See: https://lwn.net/Articles/542629/
    //
    for (i = 0; i < NUM_SRC_SKTS; i += 1) {
        skt = dgram.createSocket("udp4");
        skt.bind({
            address: "localhost",
            port: 0,
            exclusive: true
        });
        skts.push(skt);
    }

    function emitOne() {
        trans_id += 1;
        emitPayload(skts, trans_id, function (err) {
            if (err) {
                console.log("Fail: " + err);
                throw err;
            }

            cnt -= 1;
            if (cnt === 0) {
                skts.forEach(function (s) {
                    s.close();
                });
            } else {
                emitOne();
            }
        });
    }

    emitOne();
}

// Entry point
main();

