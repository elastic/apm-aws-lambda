module.exports = { generateColdstartAwareMetrics };

function generateColdstartAwareMetrics(req, res, context, events, done) {
  if(res.headers["coldstart"] === "true") {
      events.emit("histogram", `with-coldstart`, res.timings.phases.firstByte);
      events.emit('counter', `with-coldstart.codes.${res.statusCode}`, 1);
  } else {
      events.emit("histogram", `without-coldstart`, res.timings.phases.firstByte);
      events.emit('counter', `without-coldstart.codes.${res.statusCode}`, 1);
  }
  return done();
}
