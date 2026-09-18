#include "bridge.h"

#include "Luau/Parser.h"
#include "Luau/Ast.h"
#include <string>
#include <sstream>
#include <vector>
#include <cstdlib>
#include <cstring>

namespace {

std::string escapeJson(const char* str, size_t len) {
    std::ostringstream ss;
    ss << '"';
    for (size_t i = 0; i < len; ++i) {
        unsigned char c = str[i];
        switch (c) {
            case '"': ss << "\\\""; break;
            case '\\': ss << "\\\\"; break;
            case '\b': ss << "\\b"; break;
            case '\f': ss << "\\f"; break;
            case '\n': ss << "\\n"; break;
            case '\r': ss << "\\r"; break;
            case '\t': ss << "\\t"; break;
            default:
                if (c < 0x20) {
                    char buf[8];
                    snprintf(buf, sizeof(buf), "\\u%04x", c);
                    ss << buf;
                } else {
                    ss << c;
                }
                break;
        }
    }
    ss << '"';
    return ss.str();
}

std::string escapeJson(const std::string& str) {
    return escapeJson(str.data(), str.size());
}

std::string serializeExpr(Luau::AstExpr* expr);
std::string serializeStat(Luau::AstStat* stat);
std::string serializeBlock(Luau::AstStatBlock* block);

std::string serializeBlock(Luau::AstStatBlock* block) {
    if (!block) return "{\"type\":\"Block\",\"body\":[]}";
    std::ostringstream ss;
    ss << "{\"type\":\"Block\",\"body\":[";
    for (size_t i = 0; i < block->body.size; ++i) {
        if (i > 0) ss << ",";
        ss << serializeStat(block->body.data[i]);
    }
    ss << "]}";
    return ss.str();
}

std::string serializeExpr(Luau::AstExpr* expr) {
    if (!expr) return "null";
    std::ostringstream ss;

    if (auto e = expr->as<Luau::AstExprGroup>()) {
        return serializeExpr(e->expr);
    }
    if (expr->as<Luau::AstExprConstantNil>()) {
        return "{\"type\":\"Nil\"}";
    }
    if (auto e = expr->as<Luau::AstExprConstantBool>()) {
        ss << "{\"type\":\"Bool\",\"value\":" << (e->value ? "true" : "false") << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprConstantNumber>()) {
        ss << "{\"type\":\"Number\",\"value\":" << e->value << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprConstantInteger>()) {
        ss << "{\"type\":\"Number\",\"value\":" << e->value << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprConstantString>()) {
        ss << "{\"type\":\"String\",\"value\":" << escapeJson(e->value.data, e->value.size) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprLocal>()) {
        ss << "{\"type\":\"LocalRef\",\"name\":" << escapeJson(e->local->name.value) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprGlobal>()) {
        ss << "{\"type\":\"GlobalRef\",\"name\":" << escapeJson(e->name.value) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprBinary>()) {
        const char* opStr = "?";
        switch (e->op) {
            case Luau::AstExprBinary::Add: opStr = "+"; break;
            case Luau::AstExprBinary::Sub: opStr = "-"; break;
            case Luau::AstExprBinary::Mul: opStr = "*"; break;
            case Luau::AstExprBinary::Div: opStr = "/"; break;
            case Luau::AstExprBinary::FloorDiv: opStr = "//"; break;
            case Luau::AstExprBinary::Mod: opStr = "%"; break;
            case Luau::AstExprBinary::Pow: opStr = "^"; break;
            case Luau::AstExprBinary::Concat: opStr = ".."; break;
            case Luau::AstExprBinary::CompareNe: opStr = "~="; break;
            case Luau::AstExprBinary::CompareEq: opStr = "=="; break;
            case Luau::AstExprBinary::CompareLt: opStr = "<"; break;
            case Luau::AstExprBinary::CompareLe: opStr = "<="; break;
            case Luau::AstExprBinary::CompareGt: opStr = ">"; break;
            case Luau::AstExprBinary::CompareGe: opStr = ">="; break;
            case Luau::AstExprBinary::And: opStr = "and"; break;
            case Luau::AstExprBinary::Or: opStr = "or"; break;
            default: opStr = "?"; break;
        }
        ss << "{\"type\":\"Binary\",\"op\":" << escapeJson(opStr)
           << ",\"left\":" << serializeExpr(e->left)
           << ",\"right\":" << serializeExpr(e->right) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprUnary>()) {
        const char* opStr = "?";
        switch (e->op) {
            case Luau::AstExprUnary::Op::Not: opStr = "not"; break;
            case Luau::AstExprUnary::Op::Minus: opStr = "-"; break;
            case Luau::AstExprUnary::Op::Len: opStr = "#"; break;
        }
        ss << "{\"type\":\"Unary\",\"op\":" << escapeJson(opStr)
           << ",\"expr\":" << serializeExpr(e->expr) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprCall>()) {
        ss << "{\"type\":\"Call\",\"self\":" << (e->self ? "true" : "false")
           << ",\"func\":" << serializeExpr(e->func) << ",\"args\":[";
        for (size_t i = 0; i < e->args.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(e->args.data[i]);
        }
        ss << "]}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprIndexName>()) {
        ss << "{\"type\":\"IndexName\",\"expr\":" << serializeExpr(e->expr)
           << ",\"index\":" << escapeJson(e->index.value) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprIndexExpr>()) {
        ss << "{\"type\":\"IndexExpr\",\"expr\":" << serializeExpr(e->expr)
           << ",\"index_expr\":" << serializeExpr(e->index) << "}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprTable>()) {
        ss << "{\"type\":\"Table\",\"items\":[";
        for (size_t i = 0; i < e->items.size; ++i) {
            if (i > 0) ss << ",";
            const auto& item = e->items.data[i];
            ss << "{\"kind\":";
            switch (item.kind) {
                case Luau::AstExprTable::Item::Kind::List:
                    ss << "\"List\",\"val\":" << serializeExpr(item.value);
                    break;
                case Luau::AstExprTable::Item::Kind::Record:
                    ss << "\"Record\",\"key\":" << serializeExpr(item.key) << ",\"val\":" << serializeExpr(item.value);
                    break;
                case Luau::AstExprTable::Item::Kind::General:
                    ss << "\"General\",\"key\":" << serializeExpr(item.key) << ",\"val\":" << serializeExpr(item.value);
                    break;
            }
            ss << "}";
        }
        ss << "]}";
        return ss.str();
    }
    if (auto e = expr->as<Luau::AstExprFunction>()) {
        ss << "{\"type\":\"Function\",\"has_self\":" << (e->self ? "true" : "false")
           << ",\"vararg\":" << (e->vararg ? "true" : "false")
           << ",\"params\":[";
        size_t paramCount = 0;
        if (e->self) {
            ss << escapeJson(e->self->name.value);
            paramCount++;
        }
        for (size_t i = 0; i < e->args.size; ++i) {
            if (paramCount > 0) ss << ",";
            ss << escapeJson(e->args.data[i]->name.value);
            paramCount++;
        }
        ss << "],\"block\":" << serializeBlock(e->body) << "}";
        return ss.str();
    }
    if (expr->as<Luau::AstExprVarargs>()) {
        return "{\"type\":\"Vararg\"}";
    }

    return "{\"type\":\"UnknownExpr\"}";
}

std::string serializeStat(Luau::AstStat* stat) {
    if (!stat) return "null";
    std::ostringstream ss;

    if (auto s = stat->as<Luau::AstStatBlock>()) {
        return serializeBlock(s);
    }
    if (auto s = stat->as<Luau::AstStatLocal>()) {
        ss << "{\"type\":\"LocalStat\",\"vars\":[";
        for (size_t i = 0; i < s->vars.size; ++i) {
            if (i > 0) ss << ",";
            ss << escapeJson(s->vars.data[i]->name.value);
        }
        ss << "],\"values\":[";
        for (size_t i = 0; i < s->values.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(s->values.data[i]);
        }
        ss << "]}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatLocalFunction>()) {
        ss << "{\"type\":\"LocalStat\",\"vars\":[" << escapeJson(s->name->name.value)
           << "],\"values\":[" << serializeExpr(s->func) << "]}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatFunction>()) {
        ss << "{\"type\":\"AssignStat\",\"var_nodes\":[" << serializeExpr(s->name)
           << "],\"values\":[" << serializeExpr(s->func) << "]}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatAssign>()) {
        ss << "{\"type\":\"AssignStat\",\"var_nodes\":[";
        for (size_t i = 0; i < s->vars.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(s->vars.data[i]);
        }
        ss << "],\"values\":[";
        for (size_t i = 0; i < s->values.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(s->values.data[i]);
        }
        ss << "]}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatCompoundAssign>()) {
        const char* opStr = "+";
        switch (s->op) {
            case Luau::AstExprBinary::Add: opStr = "+"; break;
            case Luau::AstExprBinary::Sub: opStr = "-"; break;
            case Luau::AstExprBinary::Mul: opStr = "*"; break;
            case Luau::AstExprBinary::Div: opStr = "/"; break;
            case Luau::AstExprBinary::FloorDiv: opStr = "//"; break;
            case Luau::AstExprBinary::Mod: opStr = "%"; break;
            case Luau::AstExprBinary::Pow: opStr = "^"; break;
            case Luau::AstExprBinary::Concat: opStr = ".."; break;
            default: break;
        }
        ss << "{\"type\":\"AssignStat\",\"var_nodes\":[" << serializeExpr(s->var)
           << "],\"values\":[{\"type\":\"Binary\",\"op\":" << escapeJson(opStr)
           << ",\"left\":" << serializeExpr(s->var) << ",\"right\":" << serializeExpr(s->value) << "}]}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatExpr>()) {
        ss << "{\"type\":\"ExprStat\",\"expr\":" << serializeExpr(s->expr) << "}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatIf>()) {
        ss << "{\"type\":\"IfStat\",\"condition\":" << serializeExpr(s->condition)
           << ",\"then\":" << serializeBlock(s->thenbody)
           << ",\"else\":" << (s->elsebody ? serializeStat(s->elsebody) : "null") << "}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatFor>()) {
        ss << "{\"type\":\"ForStat\",\"var\":" << escapeJson(s->var->name.value)
           << ",\"from\":" << serializeExpr(s->from)
           << ",\"to\":" << serializeExpr(s->to)
           << ",\"step\":" << (s->step ? serializeExpr(s->step) : "null")
           << ",\"block\":" << serializeBlock(s->body) << "}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatForIn>()) {
        ss << "{\"type\":\"ForInStat\",\"vars\":[";
        for (size_t i = 0; i < s->vars.size; ++i) {
            if (i > 0) ss << ",";
            ss << escapeJson(s->vars.data[i]->name.value);
        }
        ss << "],\"values\":[";
        for (size_t i = 0; i < s->values.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(s->values.data[i]);
        }
        ss << "],\"block\":" << serializeBlock(s->body) << "}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatWhile>()) {
        ss << "{\"type\":\"WhileStat\",\"condition\":" << serializeExpr(s->condition)
           << ",\"block\":" << serializeBlock(s->body) << "}";
        return ss.str();
    }
    if (auto s = stat->as<Luau::AstStatRepeat>()) {
        ss << "{\"type\":\"RepeatStat\",\"condition\":" << serializeExpr(s->condition)
           << ",\"block\":" << serializeBlock(s->body) << "}";
        return ss.str();
    }
    if (stat->as<Luau::AstStatBreak>()) {
        return "{\"type\":\"BreakStat\"}";
    }
    if (stat->as<Luau::AstStatContinue>()) {
        return "{\"type\":\"ContinueStat\"}";
    }
    if (auto s = stat->as<Luau::AstStatReturn>()) {
        ss << "{\"type\":\"ReturnStat\",\"values\":[";
        for (size_t i = 0; i < s->list.size; ++i) {
            if (i > 0) ss << ",";
            ss << serializeExpr(s->list.data[i]);
        }
        ss << "]}";
        return ss.str();
    }

    return "{\"type\":\"UnknownStat\"}";
}

} // namespace

extern "C" {

char* LuauParseToJSON(const char* source) {
    if (!source) return nullptr;

    Luau::Allocator allocator;
    Luau::AstNameTable names(allocator);
    Luau::ParseOptions options;

    Luau::ParseResult result = Luau::Parser::parse(source, strlen(source), names, allocator, options);

    std::string outputJson;
    if (!result.errors.empty()) {
        std::ostringstream ss;
        ss << "{\"error\":" << escapeJson(result.errors.front().getMessage()) << "}";
        outputJson = ss.str();
    } else {
        std::ostringstream ss;
        ss << "{\"root\":" << serializeBlock(result.root) << "}";
        outputJson = ss.str();
    }

    char* res = static_cast<char*>(malloc(outputJson.size() + 1));
    if (res) {
        memcpy(res, outputJson.data(), outputJson.size());
        res[outputJson.size()] = '\0';
    }
    return res;
}

void LuauFreeString(char* str) {
    if (str) {
        free(str);
    }
}

}
