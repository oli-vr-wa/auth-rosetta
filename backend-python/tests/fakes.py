class FakeUsers:
    def __init__(self):
        self._users = {}

    def get_by_email(self, email):
        return self._users.get(email)

    def try_add(self, user):
        if user.email in self._users:
            return False
        self._users[user.email] = user
        return True

    def save(self):
        pass

class RacyUsers(FakeUsers):
    """Two requests both passed the 'already exists? check; the database says no to the loser."""

    def get_by_email(self, email):
        return None

    def try_add(self, user):
        return False


class FakeRevoked:
    def __init__(self):
        self.jtis = {}

    def revoke(self, jti, expires_at):
        self.jtis[jti] = expires_at

    def is_revoked(self, jti):
        return jti in self.jtis