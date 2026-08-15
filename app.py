from flask import Flask, request, jsonify
import yaml, requests

app = Flask(__name__)

@app.route('/health')
def health():
    return jsonify({"status": "ok", "service": "payments-api"})

@app.route('/payments', methods=['POST'])
def payments():
    data = request.get_json()
    return jsonify({"id": "pay_001", "status": "processed", "amount": data.get("amount")})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
