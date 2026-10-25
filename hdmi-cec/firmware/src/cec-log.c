#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#include "FreeRTOS.h"
#include "message_buffer.h"
#include "task.h"

#include "pico-cec/config.h"
#include "pico-cec/util.h"

#include "cec-frame.h"
#include "cec-id.h"
#include "cec-log.h"
#include "cec-user.h"

#define LOG_LINE_LENGTH (64)
#define LOG_QUEUE_LENGTH (16)
#define LOG_MB_SIZE (LOG_LINE_LENGTH * LOG_QUEUE_LENGTH)

static StaticTask_t log_task_static;
static StackType_t log_stack[LOG_STACK_SIZE];

static StaticMessageBuffer_t log_mb_static;
static MessageBufferHandle_t log_mb;
static uint8_t log_mb_storage[LOG_MB_SIZE];

static volatile bool enabled = false;

static void cec_log_task(void *param) {
  log_callback_t log = param;

  while (true) {
    char buffer[LOG_LINE_LENGTH];

    size_t bytes = xMessageBufferReceive(log_mb, buffer, sizeof(buffer), pdMS_TO_TICKS(10));
    if (bytes > 0) {
      log(buffer);
    }
  }
}

void cec_log_init(log_callback_t log) {
  log_mb = xMessageBufferCreateStatic(LOG_MB_SIZE, &log_mb_storage[0], &log_mb_static);
  enabled = false;

  xTaskCreateStatic(cec_log_task, LOG_TASK_NAME, LOG_STACK_SIZE, log, LOG_PRIORITY, &log_stack[0],
                    &log_task_static);
}

bool cec_log_enabled(void) {
  return enabled;
}

void cec_log_enable(void) {
  enabled = true;
}

void cec_log_disable(void) {
  enabled = false;
}

void cec_log_vsubmitf(const char *fmt, va_list ap) {
  if (enabled) {
    char buffer[LOG_LINE_LENGTH];

    int bytes = vsnprintf(buffer, sizeof(buffer), fmt, ap);
    if (bytes < sizeof(buffer)) {
      xMessageBufferSend(log_mb, buffer, bytes + 1, pdMS_TO_TICKS(20));
    }
  }
}

void cec_log_submitf(const char *fmt, ...) {
  va_list ap;
  va_start(ap, fmt);
  cec_log_vsubmitf(fmt, ap);
  va_end(ap);
}


void cec_log_frame(cec_frame_t *frame, bool recv) {
  if (!enabled) return;
  char data[CEC_FRAME_MAX_LEN * 3 + 1] = {0};
  unsigned len = frame->message->len;
  if (len > CEC_FRAME_MAX_LEN) len = CEC_FRAME_MAX_LEN;
  for (unsigned i = 0; i < len; ++i)
    snprintf(data + i * 3, sizeof(data) - i * 3, "%02x ", frame->message->data[i]);
  cec_log_submitf("%s ack=%u %s"_LOG_BR, recv ? "RX" : "TX", frame->ack, data);
}
