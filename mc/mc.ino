#include <Wire.h>
#include <Servo.h>

#include "mc.h"

// Magic number representing failed delta computation.
#define DELTA_FAILURE 2

int8_t encoderDelta[2][2][2][2] = {
  {  // pin1 was LOW
    {  // pin2 was LOW
      {  // pin1 is LOW
        0,  // pin2 is LOW
        1,  // pin2 is HIGH
      },
      {  // pin1 is HIGH
        -1,             // pin2 is LOW
        DELTA_FAILURE,  // pin2 is HIGH
      },
    },
    {  // pin2 was HIGH
      {  // pin1 is LOW
        -1,  // pin2 is LOW
        0,   // pin2 is HIGH
      },
      {  // pin1 is HIGH
        DELTA_FAILURE,  // pin2 is LOW
        1,              // pin2 is HIGH
      },
    },
  },
  {  // pin1 was HIGH
    {  // pin2 was LOW
      {  // pin1 is LOW
        1,              // pin2 is LOW
        DELTA_FAILURE,  // pin2 is HIGH
      },
      {  // pin1 is HIGH
        0,   // pin2 is LOW
        -1,  // pin2 is HIGH
      },
    },
    {  // pin2 was HIGH
      {  // pin1 is LOW
        DELTA_FAILURE,  // pin2 is LOW
        -1,             // pin2 is HIGH
      },
      {  // pin1 is HIGH
        1,  // pin2 is LOW
        0,  // pin2 is HIGH
      },
    },
  },
};

enum {
  PinMotorLeft = 10,
  PinMotorRight = 11,
};

int32_t encoderValue[4] = { 0 };
int previousEncoderDelta[4] = { 0 };
int previousPinValue[EncoderPins];

Servo left, right;

volatile boolean brake = false;

volatile byte i2cRegister = 0xff;  // register to read from / write to

void attachMotors(boolean attach) {
  if (attach) {
    left.attach(PinMotorLeft, 1000, 2000);
    right.attach(PinMotorRight, 1000, 2000);
  } else {
    left.detach();
    right.detach();
  }
}

void setup() {
  for (int i = MinEncoderPin; i < EncoderPins; i++) {
    pinMode(i, INPUT);
    previousPinValue[i] = (digitalRead(i) == HIGH);
  }
  Wire.begin(I2CAddress);
  Wire.onReceive(i2cReceive);
  Wire.onRequest(i2cRequest);
  attachMotors(true);
}

void loop() {
  // Specs: http://www.lynxmotion.com/p-448-quadrature-motor-encoder-wcable.aspx
  // Frequency up to 30khz gives ~33us for a single loop() cycle.
  for (int i = MinEncoderPin; i < EncoderPins; i+=2) {
    int newPin1Value = (digitalRead(i) == HIGH),
        newPin2Value = (digitalRead(i+1) == HIGH);
    int delta = encoderDelta
      [previousPinValue[i]]
      [previousPinValue[i+1]]
      [newPin1Value]
      [newPin2Value];

    if (delta == DELTA_FAILURE) {
        // If delta doesn't add up, assume missed round.
        delta = previousEncoderDelta[i] * 2;
    } else {
        previousEncoderDelta[i] = delta;
    }
    encoderValue[(i-MinEncoderPin) >> 1] += delta;

    previousPinValue[i] = newPin1Value;
    previousPinValue[i+1] = newPin2Value;
  }
  // TODO(dotdoom): check brake, millis() etc
}

void i2cReceive(int bytesReceived) {
  i2cRegister = Wire.read();
  if (bytesReceived > 1) {
    byte value8 = Wire.read();
    switch (i2cRegister) {
    case RegisterCommand:
      switch (value8) {
      case CommandBrake:
        brake = true;
        break;
      case CommandReleaseBrake:
        brake = false;
        break;
      case CommandSleep:
        attachMotors(false);
        break;
      case CommandWake:
        attachMotors(true);
        break;
      }
      break;
    case RegisterMotorLeft:
      left.write(value8);
      break;
    case RegisterMotorRight:
      right.write(value8);
      break;
    }
  }
  // onReceive will not be invoked unless rxBuffer is empty.
  // Clean it up manually.
  while (Wire.available()) { Wire.read(); }
}

void i2cRequest() {
  byte encoder = (byte)(i2cRegister-1);
  if (encoder <= 3) {
    Wire.write(reinterpret_cast<byte*>(&encoderValue[encoder]),
        sizeof(encoderValue[encoder]));
  }
}
