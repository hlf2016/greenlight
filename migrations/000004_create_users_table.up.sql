CREATE TABLE IF NOT EXISTS users (
     id bigserial PRIMARY KEY,
     created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
     name text NOT NULL,
     email citext UNIQUE NOT NULL,
     password_hash bytea NOT NULL,
     activated bool NOT NULL,
     version integer NOT NULL DEFAULT 1
);

-- 电子邮件列的类型是 citext（大小写不敏感文本）。这种类型存储的文本数据与输入的数据完全一致，不会改变大小写，但与数据的比较总是不区分大小写......包括在相关索引上的查找。
-- 我们还在电子邮件列上设置了 UNIQUE 约束。与 citext 类型相结合，这意味着数据库中没有两行的电子邮件值是相同的，即使它们的情况不同。这基本上强制执行了数据库级的业务规则，即不能有两个用户的电子邮件地址相同。
-- password_hash 列的类型是 bytea（二进制字符串）。在这一列中，我们将存储使用 bcrypt 生成的用户密码的单向散列，而不是明文密码本身。
-- ACTIVATED列存储一个布尔值，以指示用户帐户是否处于“活动”状态。在创建新用户时，我们将默认将其设置为False，并要求用户在将其设置为True之前确认其电子邮件地址。
-- 我们还加入了一个 version 列，每次更新用户记录时都会递增。这样，在更新用户记录时，我们就可以使用乐观锁定来防止出现竞赛条件，就像我们在书中早些时候对电影所做的那样。
