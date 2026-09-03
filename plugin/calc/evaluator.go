package plugin_calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// evaluate 计算数学表达式
func evaluate(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}

	// 预处理：替换常量
	expr = strings.ReplaceAll(expr, "pi", fmt.Sprintf("%f", math.Pi))
	expr = strings.ReplaceAll(expr, "e", fmt.Sprintf("%f", math.E))

	// 解析并计算
	parser := &parser{
		expr: expr,
		pos:  0,
	}
	return parser.parseExpression()
}

// formatResult 格式化结果
func formatResult(value float64) string {
	// 如果是整数，显示为整数
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	// 否则显示为浮点数，最多6位小数
	return fmt.Sprintf("%.6g", value)
}

// parser 表达式解析器
type parser struct {
	expr string
	pos  int
}

// 当前字符
func (p *parser) current() byte {
	if p.pos >= len(p.expr) {
		return 0
	}
	return p.expr[p.pos]
}

// 前进一个字符
func (p *parser) advance() {
	p.pos++
}

// 跳过空白字符
func (p *parser) skipWhitespace() {
	for p.pos < len(p.expr) && unicode.IsSpace(rune(p.current())) {
		p.advance()
	}
}

// 解析表达式（处理加减）
func (p *parser) parseExpression() (float64, error) {
	result, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		op := p.current()
		if op != '+' && op != '-' {
			break
		}
		p.advance()

		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}

		if op == '+' {
			result += right
		} else {
			result -= right
		}
	}

	return result, nil
}

// 解析项（处理乘除模）
func (p *parser) parseTerm() (float64, error) {
	result, err := p.parsePower()
	if err != nil {
		return 0, err
	}

	for {
		p.skipWhitespace()
		op := p.current()
		if op != '*' && op != '/' && op != '%' {
			break
		}
		p.advance()

		right, err := p.parsePower()
		if err != nil {
			return 0, err
		}

		switch op {
		case '*':
			result *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			result /= right
		case '%':
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			result = math.Mod(result, right)
		}
	}

	return result, nil
}

// 解析幂运算
func (p *parser) parsePower() (float64, error) {
	result, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	p.skipWhitespace()
	if p.current() == '^' {
		p.advance()
		right, err := p.parsePower() // 右结合
		if err != nil {
			return 0, err
		}
		result = math.Pow(result, right)
	}

	return result, nil
}

// 解析因子（数字、函数、括号）
func (p *parser) parseFactor() (float64, error) {
	p.skipWhitespace()

	// 处理负号
	if p.current() == '-' {
		p.advance()
		val, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}

	// 处理正号
	if p.current() == '+' {
		p.advance()
		return p.parseFactor()
	}

	// 处理括号
	if p.current() == '(' {
		p.advance()
		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipWhitespace()
		if p.current() != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.advance()
		return result, nil
	}

	// 处理函数和数字
	if unicode.IsLetter(rune(p.current())) {
		return p.parseFunction()
	}

	return p.parseNumber()
}

// 解析数字
func (p *parser) parseNumber() (float64, error) {
	p.skipWhitespace()
	start := p.pos

	// 读取数字
	for p.pos < len(p.expr) && (unicode.IsDigit(rune(p.current())) || p.current() == '.') {
		p.advance()
	}

	if start == p.pos {
		return 0, fmt.Errorf("expected number at position %d", p.pos)
	}

	numStr := p.expr[start:p.pos]
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", numStr)
	}

	return value, nil
}

// 解析函数
func (p *parser) parseFunction() (float64, error) {
	start := p.pos

	// 读取函数名
	for p.pos < len(p.expr) && unicode.IsLetter(rune(p.current())) {
		p.advance()
	}

	funcName := p.expr[start:p.pos]
	p.skipWhitespace()

	// 函数必须后跟括号
	if p.current() != '(' {
		return 0, fmt.Errorf("expected '(' after function %s", funcName)
	}
	p.advance()

	// 解析参数
	arg, err := p.parseExpression()
	if err != nil {
		return 0, err
	}

	p.skipWhitespace()
	if p.current() != ')' {
		return 0, fmt.Errorf("missing closing parenthesis for function %s", funcName)
	}
	p.advance()

	// 计算函数值
	switch strings.ToLower(funcName) {
	case "sin":
		return math.Sin(arg), nil
	case "cos":
		return math.Cos(arg), nil
	case "tan":
		return math.Tan(arg), nil
	case "sqrt":
		if arg < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(arg), nil
	case "abs":
		return math.Abs(arg), nil
	case "log":
		if arg <= 0 {
			return 0, fmt.Errorf("log of non-positive number")
		}
		return math.Log10(arg), nil
	case "ln":
		if arg <= 0 {
			return 0, fmt.Errorf("ln of non-positive number")
		}
		return math.Log(arg), nil
	case "exp":
		return math.Exp(arg), nil
	case "floor":
		return math.Floor(arg), nil
	case "ceil":
		return math.Ceil(arg), nil
	case "round":
		return math.Round(arg), nil
	default:
		return 0, fmt.Errorf("unknown function: %s", funcName)
	}
}
